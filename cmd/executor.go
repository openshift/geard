package cmd

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/pkg/logstreamer"
	"github.com/openshift/geard/transport"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
)

type check interface {
	Check() error
}

type FuncBulk func(...Locator) jobs.Job
type FuncSerial func(Locator) jobs.Job
type FuncReact func(*CliJobResponse, io.Writer, interface{})

// An executor runs a number of local or remote jobs in
// parallel or sequentially.  You must set either .Group
// or .Serial
type Executor struct {
	On []Locator
	// Given a set of locators on the same server, create one
	// job that represents all ids.
	Group FuncBulk
	// Given a set of locators on the same server, create one
	// job per locator.
	Serial FuncSerial
	// The stream to output to, will be set to DevNull by default
	Output io.Writer
	// Optional: specify an initializer for local execution
	LocalInit FuncInit
	// Optional: respond to successful calls
	OnSuccess FuncReact
	// Optional: respond to errors when they occur
	OnFailure FuncReact

	Transport transport.Transport
}

// Invoke the appropriate job on each server and return the set of data
func (e Executor) Gather() (data []interface{}, failures []error) {
	if e.Output == nil {
		e.Output = ioutil.Discard
	}
	data = make([]interface{}, 0, len(e.On))
	failures = make([]error, 0)

	responses, err := e.run(true)
	if err != nil {
		failures = append(failures, err)
		return
	}
	for i := range responses {
		if responses[i].Error != nil {
			failures = append(failures, responses[i].Error)
			continue
		}
		datum := responses[i].Data
		if datum == nil {
			failures = append(failures, errors.New(fmt.Sprintf("Response %d did not return any data", i)))
			continue
		}
		data = append(data, datum)
	}
	return
}

// Stream responses from all servers to output
func (e Executor) Stream() (failures []error) {
	if e.Output == nil {
		e.Output = ioutil.Discard
	}
	failures = make([]error, 0)

	responses, err := e.run(false)
	if err != nil {
		failures = append(failures, err)
		return
	}
	for i := range responses {
		if responses[i].Error != nil {
			failures = append(failures, responses[i].Error)
		}
	}
	return
}

func (e Executor) StreamAndExit() {
	if errors := e.Stream(); len(errors) > 0 {
		if e.OnFailure == nil {
			for i := range errors {
				fmt.Fprintf(os.Stderr, "Error: %s\n", errors[i].Error())
			}
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func (e *Executor) run(gather bool) ([]*CliJobResponse, error) {
	on := e.On
	local, remote := Locators(on).Group()
	single := len(on) == 1
	responses := []*CliJobResponse{}

	// Check each job first, return the first error (coding bugs)
	localJobs := e.jobs(local)
	if err := localJobs.check(); err != nil {
		return responses, err
	}
	remoteJobs := make([]remoteJobSet, len(remote))
	for i := range remote {
		jobs := e.jobs(remote[i])
		if err := jobs.check(); err != nil {
			return responses, err
		}
		remotes, err := jobs.remotes()
		if err != nil {
			return responses, err
		}
		remoteJobs[i] = remotes
	}

	// Perform local initialization
	if len(local) > 0 && e.LocalInit != nil {
		if err := e.LocalInit(); err != nil {
			return responses, err
		}
	}

	respch := make(chan *CliJobResponse, len(on))
	tasks := &sync.WaitGroup{}
	stdout := log.New(e.Output, "", 0)

	// Execute the local jobs in serial (can parallelize later)
	if len(localJobs) > 0 {
		tasks.Add(1)
		go func() {
			w := logstreamer.NewLogstreamer(stdout, "local ", false)
			defer w.Close()
			defer tasks.Done()

			for _, job := range localJobs {
				response := &CliJobResponse{Output: w, Gather: gather}
				job.Execute(response)
				respch <- e.react(response, w, job)
			}
		}()
	}

	// Executes jobs against each remote server in parallel
	for i := range remote {
		ids := remote[i]
		allJobs := remoteJobs[i]
		host := ids[0].HostIdentity()
		locator := ids[0].(http.RemoteLocator)

		tasks.Add(1)
		go func() {
			w := logstreamer.NewLogstreamer(stdout, prefixUnless(host+" ", single), false)
			logger := log.New(w, "", 0)
			defer w.Close()
			defer tasks.Done()

			for _, job := range allJobs {
				response := &CliJobResponse{Output: w, Gather: gather}
				dispatcher, err := e.Transport.NewDispatcher(locator, logger)
				if err != nil {
					response = &CliJobResponse{
						Error: jobs.SimpleJobError{jobs.JobResponseError, fmt.Sprintf("Unable to create transport: %s", err.Error())},
					}
					respch <- e.react(response, w, job)
					continue
				}

				if err := dispatcher.Dispatch(job, response); err != nil {
					// set an explicit error
					response = &CliJobResponse{
						Error: jobs.SimpleJobError{jobs.JobResponseError, fmt.Sprintf("The server did not respond correctly: %s", err.Error())},
					}
				}
				respch <- e.react(response, w, job)
			}
		}()
	}

	tasks.Wait()
Response:
	for {
		select {
		case resp := <-respch:
			responses = append(responses, resp)
		default:
			break Response
		}
	}

	return responses, nil
}

func (e *Executor) react(response *CliJobResponse, w io.Writer, job interface{}) *CliJobResponse {
	if response.Error != nil && e.OnFailure != nil {
		e.OnFailure(response, w, job)
	}
	if response.Error == nil && e.OnSuccess != nil {
		e.OnSuccess(response, w, job)
	}
	return response
}

// Find all jobs that apply for these locators.
func (e *Executor) jobs(on []Locator) jobSet {
	if len(on) == 0 {
		return jobSet{}
	}
	if e.Group != nil {
		return jobSet{e.Group(on...)}
	}
	if e.Serial != nil {
		jobs := make(jobSet, 0, len(on))
		for i := range on {
			jobs = append(jobs, e.Serial(on[i]))
		}
		return jobs
	}
	return jobSet{}
}

type jobSet []jobs.Job
type remoteJobSet []transport.RemoteExecutable

func (jobs jobSet) check() error {
	for i := range jobs {
		job := jobs[i]
		if check, ok := job.(check); ok {
			if err := check.Check(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (jobs jobSet) remotes() (remotes remoteJobSet, err error) {
	remotes = make(remoteJobSet, 0, len(remotes))
	for i := range jobs {
		job := jobs[i]
		remotable, ok := job.(transport.RemoteExecutable)
		if !ok {
			err = errors.New(fmt.Sprintf("Unable to run this action (%+v) against a remote server", reflect.TypeOf(job)))
			return
		}
		remotes = append(remotes, remotable)
	}
	return
}

func Fail(code int, format string, other ...interface{}) {
	fmt.Fprintf(os.Stderr, format, other...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(code)
}

func prefixUnless(prefix string, cond bool) string {
	if cond {
		return ""
	}
	return prefix
}
