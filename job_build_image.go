package geard

import (
	"fmt"
	"github.com/smarterclayton/go-systemd/dbus"
	"io"
	"log"
	"net/http"
	"reflect"
	"time"
)

type buildImageJobRequest struct {
	jobRequest
	Source    string
	BaseImage string
	Tag       string
	Output    io.Writer
}

func (j *buildImageJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Processing build-image request:\n")
	// TODO: download source, add bind-mount

	unitName := j.RequestId.UnitNameForBuild()
	unitDescription := fmt.Sprintf("Builder for %s", j.Tag)

	stdout, err := ProcessLogsForUnit(unitName)
	if err != nil {
		stdout = emptyReader
		fmt.Fprintf(j.Output, "Unable to fetch build logs: %s\n", err.Error())
	}
	defer stdout.Close()
	go io.Copy(j.Output, stdout)

	conn, errc := NewSystemdConnection()
	if errc != nil {
		log.Print("job_create_repository:", errc)
		fmt.Fprintf(j.Output, "Unable to watch start status", errc)
		return
	}
	//defer conn.Close()
	if err := conn.Subscribe(); err != nil {
		log.Print("job_create_repository:", err)
		fmt.Fprintf(j.Output, "Unable to watch start status", errc)
		return
	}
	defer conn.Unsubscribe()

	// make subscription global for efficiency
	changes, errch := conn.SubscribeUnitsCustom(1*time.Second, 2,
		func(s1 *dbus.UnitStatus, s2 *dbus.UnitStatus) bool {
			return true
		},
		func(unit string) bool {
			return unit != unitName
		})

	fmt.Fprintf(j.Output, "Running sti build unit: %s\n", unitName)

	status, err := SystemdConnection().StartTransientUnit(
		unitName,
		"fail",
		dbus.PropExecStart([]string{
			"/usr/bin/docker", "run",
			"-rm",
			"-v", "/var/run:/var/run",
			"-t", "pmorie/sti-builder",
			"sti", "build", j.Source, j.BaseImage, j.Tag,
		}, true),
		dbus.PropDescription(unitDescription),
		dbus.PropRemainAfterExit(true),
	)

	if err != nil {
		errType := reflect.TypeOf(err)
		fmt.Fprintf(j.Output, "Unable to start build container for this image due to (%s): %s\n", errType, err.Error())
		return
	} else if status != "done" {
		fmt.Fprintf(j.Output, "Build did not complete successfully: %s\n", status)
	} else {
		fmt.Fprintf(j.Output, "Sti build is running\n")
	}

	if flusher, ok := j.Output.(http.Flusher); ok {
		flusher.Flush()
	}

wait:
	for {
		select {
		case c := <-changes:
			if changed, ok := c[unitName]; ok {
				if changed.SubState != "running" {
					fmt.Fprintf(j.Output, "Build completed\n", changed.SubState)
					break wait
				}
			}
		case err := <-errch:
			fmt.Fprintf(j.Output, "Error %+v\n", err)
		case <-time.After(10 * time.Second):
			log.Print("job_build_image:", "timeout")
			break wait
		}
	}

	stdout.Close()
}
