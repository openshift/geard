package cmd

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/pkg/logstreamer"
	"github.com/openshift/geard/transport"
	"io"
	"io/ioutil"
	"log"
	"os"
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
	On Locators
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
	// Optional: a way to transport a job to a remote server. If not
	// specified remote locators will fail
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
	local, remote := on.Group()
	single := len(on) == 1
	responses := []*CliJobResponse{}

	// Check each job first, return the first error (coding bugs)
	localJobs := e.jobs(local)
	if err := localJobs.check(); err != nil {
		return responses, err
	}
	remoteJobs := make([][]remoteJob, len(remote))
	for i := range remote {
		locator := remote[i]
		jobs := e.jobs(locator)
		if err := jobs.check(); err != nil {
			return responses, err
		}
		remotes := make([]remoteJob, len(jobs))
		for j := range jobs {
			remote, err := e.Transport.RemoteJobFor(locator[0].TransportLocator(), jobs[j])
			if err != nil {
				return responses, err
			}
			remotes[j] = remoteJob{remote, jobs[j], locator[0]}
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
			w := logstreamer.NewLogstreamer(stdout, prefixUnless("local ", single), false)
			defer w.Close()
			defer tasks.Done()

			for _, job := range localJobs {
				response := &CliJobResponse{Output: w, Gather: gather}
				job.Execute(response)
				respch <- e.react(response, w, job)
			}
		}()
	}

	// Executes jobs against each remote server in parallel (could parallel to each server if necessary)
	for i := range remote {
		ids := remote[i]
		allJobs := remoteJobs[i]
		host := ids[0].TransportLocator()

		tasks.Add(1)
		go func() {
			w := logstreamer.NewLogstreamer(stdout, prefixUnless(host.String()+" ", single), false)
			defer w.Close()
			defer tasks.Done()

			for _, job := range allJobs {
				response := &CliJobResponse{Output: w, Gather: gather}
				job.Execute(response)
				respch <- e.react(response, w, job.Original)
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
type remoteJob struct {
	jobs.Job
	Original jobs.Job
	Locator  Locator
}

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
