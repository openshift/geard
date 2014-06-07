package linux

import (
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/openshift/geard/containers"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/geard/utils"
	"github.com/openshift/go-systemd/dbus"
)

type runContainer struct {
	*cjobs.RunContainerRequest
	systemd systemd.Systemd
}

func (j *runContainer) UnitCommand() []string {
	command := []string{
		"/usr/bin/docker", "run",
		"--rm",
	}
	if j.Command != "" {
		command = append(command, "--entrypoint", j.Command)
	}
	if j.Image != "" {
		command = append(command, "-t", j.Image)
	}
	if len(j.Arguments) > 0 {
		command = append(command, j.Arguments...)
	}
	return command
}

func (j *runContainer) Execute(resp jobs.Response) {
	command := j.UnitCommand()
	unitName := containers.JobIdentifier(j.Name).UnitNameFor()
	unitDescription := fmt.Sprintf("Execute image '%s': %s %s", j.Image, j.Command, strings.Join(command, " "))

	var (
		stdout  io.ReadCloser
		changes <-chan map[string]*dbus.UnitStatus
		errch   <-chan error
	)

	if resp.StreamResult() {
		r, err := systemd.ProcessLogsForUnit(unitName)
		if err != nil {
			r = utils.EmptyReader
			log.Printf("run_container: Unable to fetch container run logs: %s, %+v", err.Error(), err)
		}
		defer r.Close()

		conn, errc := systemd.NewConnection()
		if errc != nil {
			log.Print("run_container:", errc)
			return
		}

		if err := conn.Subscribe(); err != nil {
			log.Print("run_container:", err)
			return
		}
		defer conn.Unsubscribe()

		// make subscription global for efficiency
		c, ech := conn.SubscribeUnitsCustom(1*time.Second, 2,
			func(s1 *dbus.UnitStatus, s2 *dbus.UnitStatus) bool {
				return true
			},
			func(unit string) bool {
				return unit != unitName
			})

		stdout = r
		changes = c
		errch = ech
	}

	log.Printf("run_container: Running container %s", unitName)

	status, err := systemd.Connection().StartTransientUnit(
		unitName,
		"fail",
		dbus.PropExecStart(command, true),
		dbus.PropDescription(unitDescription),
		dbus.PropRemainAfterExit(true),
		dbus.PropSlice("container.slice"),
	)

	switch {
	case err != nil:
		errType := reflect.TypeOf(err)
		resp.Failure(jobs.SimpleError{jobs.ResponseError, fmt.Sprintf("Unable to start container execution due to (%s): %s", errType, err.Error())})
		return
	case status != "done":
		resp.Failure(jobs.SimpleError{jobs.ResponseError, fmt.Sprintf("Start did not complete successfully: %s", status)})
		return
	case stdout == nil:
		resp.Success(jobs.ResponseOk)
		return
	}

	w := resp.SuccessWithWrite(jobs.ResponseAccepted, true, false)
	go io.Copy(w, stdout)

wait:
	for {
		select {
		case c := <-changes:
			if changed, ok := c[unitName]; ok {
				if changed.SubState != "running" {
					break wait
				}
			}
		case err := <-errch:
			fmt.Fprintf(w, "Error %+v\n", err)
		case <-time.After(1 * time.Minute):
			log.Print("run_container:", "timeout")
			break wait
		}
	}

	stdout.Close()
}
