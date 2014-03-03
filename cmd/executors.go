package cmd

import (
	"fmt"
	"github.com/smarterclayton/cobra"
	"github.com/smarterclayton/geard/http"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/pkg/logstreamer"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
)

// A simple executor that groups each remote / local system and simultaneous streams
// output to the client.  Exits with 0 if all succeeded or the first error code.
func run(cmd *cobra.Command, localInit func(), init func(...Locator) jobs.Job, on ...Locator) {
	exitch := make(chan int, len(on))
	stdout := log.New(cmd.Out(), "", log.Ldate|log.Ltime)
	wg := &sync.WaitGroup{}
	local, remote := Locators(on).Group()

	if len(local) > 0 {
		localInit()
		go func() {
			wg.Add(1)
			lstdout := logstreamer.NewLogstreamer(stdout, "local ", false)
			defer lstdout.Close()
			defer wg.Done()

			job := init(local...)
			response := &CliJobResponse{stdout: lstdout, stderr: lstdout}
			job.Execute(response)
			if response.exitCode != 0 {
				if response.message == "" {
					response.message = "Command failed"
				}
				fmt.Fprintf(lstdout, response.message)
			}
			exitch <- response.exitCode
		}()
	}

	for i := range remote {
		go func() {
			wg.Add(1)
			w := logstreamer.NewLogstreamer(stdout, remote[i][0].Identity()+" ", false)
			defer w.Close()
			defer wg.Done()

			job := init(local...)
			code := 0
			if remotable, ok := job.(http.RemoteJob); ok {
				fmt.Fprintf(w, "Executing %d %v", i, remotable)
			} else {
				fmt.Fprintf(w, "Unable to run this action (%+v) against a remote server", reflect.TypeOf(job))
				code = 1
			}
			exitch <- code
		}()
	}

	var code int
	select {
	case code = <-exitch:
	}
	wg.Wait()
	os.Exit(code)
}

// A simple executor that runs commands on different servers in parallel but invokes
// one job per identifier.
func runEach(cmd *cobra.Command, localInit func(), init func(Locator) jobs.Job, on ...Locator) {
	exitch := make(chan int, len(on))
	stdout := log.New(cmd.Out(), "", log.Ldate|log.Ltime)
	wg := &sync.WaitGroup{}
	local, remote := Locators(on).Group()

	if len(local) > 0 {
		localInit()
		go func() {
			wg.Add(1)
			lstdout := logstreamer.NewLogstreamer(stdout, "local ", false)
			defer lstdout.Close()
			defer wg.Done()

			code := 0
			for i := range local {
				job := init(local[i])
				response := &CliJobResponse{stdout: lstdout, stderr: lstdout}
				job.Execute(response)
				if response.exitCode != 0 {
					if response.message == "" {
						response.message = "Command failed"
					}
					fmt.Fprintf(lstdout, response.message)
					code = response.exitCode
				}
			}
			exitch <- code
		}()
	}

	for i := range remote {
		go func() {
			wg.Add(1)
			ids := remote[i]
			host := ids[0].Identity()
			w := logstreamer.NewLogstreamer(stdout, host+" ", false)
			defer w.Close()
			defer wg.Done()

			locator := ids[0].(http.RemoteLocator)
			dispatcher := http.NewHttpDispatcher(locator)

			code := 0
			for j := range ids {
				job := init(ids[j])
				if remotable, ok := job.(http.RemoteExecutable); ok {
					fmt.Fprintf(w, "Executing %v", remotable)
					dispatcher.Dispatch(remotable)
				} else {
					fmt.Fprintf(w, "Unable to run this action (%+v) against a remote server", reflect.TypeOf(job))
					if code == 0 {
						code = 1
					}
				}
			}
			exitch <- code
		}()
	}

	var code int
	select {
	case code = <-exitch:
	}
	wg.Wait()
	os.Exit(code)
}

func fail(code int, format string, other ...interface{}) {
	fmt.Fprintf(os.Stderr, format, other...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(code)
}
