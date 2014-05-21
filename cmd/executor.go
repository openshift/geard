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
type JobRequest interface{}

type FuncBulk func(...Locator) JobRequest
type FuncSerial func(Locator) JobRequest
type FuncReact func(*CliJobResponse, io.Writer, JobRequest)

// An executor runs a number of local or remote jobs in
// parallel or sequentially.  You must set either .Group
// or .Serial
type Executor struct {
	// An interface for converting requests into jobs.
	Transport transport.Transport
	// The set of destinations to act on.
	On Locators
	// Given a set of locators on the same server, create one
	// job that represents all ids.
	Group FuncBulk
	// Given a set of locators on the same server, create one
	// job per locator.
	Serial FuncSerial
	// The stream to output to, will be set to DevNull by default
	Output io.Writer
	// Optional: respond to successful calls
	OnSuccess FuncReact
	// Optional: respond to errors when they occur
	OnFailure FuncReact
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
	remote := on.Group()
	single := len(on) == 1
	responses := []*CliJobResponse{}

	// Check each job first, return the first error (coding bugs)
	byDestination := make([]requestedJobs, len(remote))
	for i := range remote {
		group := remote[i]
		jobs := e.requests(group)
		if err := jobs.check(); err != nil {
			return responses, err
		}
		byDestination[i] = jobs
		for j := range jobs {
			remote, err := e.Transport.RemoteJobFor(jobs[j].Locator.TransportLocator(), jobs[j].Request)
			if err != nil {
				return responses, err
			}
			byDestination[i][j].Job = remote
		}
	}

	respch := make(chan *CliJobResponse, len(on))
	tasks := &sync.WaitGroup{}
	stdout := log.New(e.Output, "", 0)

	// Executes jobs against each destination in parallel, but serial on each destination.
	for i := range byDestination {
		allJobs := byDestination[i]
		host := allJobs[0].Locator.TransportLocator()

		tasks.Add(1)
		go func() {
			w := logstreamer.NewLogstreamer(stdout, prefixUnless(host.String()+" ", single), false)
			defer w.Close()
			defer tasks.Done()

			for _, job := range allJobs {
				response := &CliJobResponse{Output: w, Gather: gather}
				job.Job.Execute(response)
				respch <- e.react(response, w, job.Request)
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

func (e *Executor) react(response *CliJobResponse, w io.Writer, job JobRequest) *CliJobResponse {
	if response.Error != nil && e.OnFailure != nil {
		e.OnFailure(response, w, job)
	}
	if response.Error == nil && e.OnSuccess != nil {
		e.OnSuccess(response, w, job)
	}
	return response
}

// Find all jobs that apply for these locators.
func (e *Executor) requests(on []Locator) requestedJobs {
	if (e.Serial == nil && e.Group == nil) || (e.Serial != nil && e.Group != nil) {
		panic("Executor requires one of Group or Serial set")
	}

	if len(on) == 0 {
		return requestedJobs{}
	}
	if e.Group != nil {
		return requestedJobs{requestedJob{Request: e.Group(on...), Locator: on[0]}}
	}

	jobs := make(requestedJobs, 0, len(on))
	for i := range on {
		jobs = append(jobs, requestedJob{Request: e.Serial(on[i]), Locator: on[i]})
	}
	return jobs
}

type requestedJobs []requestedJob
type requestedJob struct {
	Request JobRequest
	Job     jobs.Job
	Locator Locator
}

func (jobs requestedJobs) check() error {
	for i := range jobs {
		job := jobs[i]
		if check, ok := job.Request.(check); ok {
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
