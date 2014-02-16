package geard

import (
	"fmt"
	"github.com/smarterclayton/go-systemd/dbus"
	"io"
	"log"
	"reflect"
	"time"
)

type buildImageJobRequest struct {
	JobResponse
	jobRequest
	Source    string
	BaseImage string
	Tag       string
}

func (j *buildImageJobRequest) Execute() {
	log.Printf("Starting build %s", j.Id())
	w := j.SuccessWithWrite(JobResponseAccepted, true)

	fmt.Fprintf(w, "Processing build-image request:\n")
	// TODO: download source, add bind-mount

	unitName := j.RequestId.UnitNameForBuild()
	unitDescription := fmt.Sprintf("Builder for %s", j.Tag)

	stdout, err := ProcessLogsForUnit(unitName)
	if err != nil {
		stdout = emptyReader
		log.Printf("job_build_image: Unable to fetch build logs: %s, %+v", err.Error(), err)
	}
	defer stdout.Close()

	conn, errc := NewSystemdConnection()
	if errc != nil {
		log.Print("job_create_repository:", errc)
		fmt.Fprintf(w, "Unable to watch start status", errc)
		return
	}
	//defer conn.Close()
	if err := conn.Subscribe(); err != nil {
		log.Print("job_create_repository:", err)
		fmt.Fprintf(w, "Unable to watch start status", errc)
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

	fmt.Fprintf(w, "Running sti build unit: %s\n", unitName)

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
		dbus.PropSlice("gear.slice"),
	)

	if err != nil {
		errType := reflect.TypeOf(err)
		fmt.Fprintf(w, "Unable to start build container for this image due to (%s): %s\n", errType, err.Error())
		return
	} else if status != "done" {
		fmt.Fprintf(w, "Build did not complete successfully: %s\n", status)
	} else {
		fmt.Fprintf(w, "Sti build is running\n")
	}

	ioerr := make(chan error)
	go func() {
		_, err := io.Copy(w, stdout)
		ioerr <- err
	}()

wait:
	for {
		select {
		case c := <-changes:
			if changed, ok := c[unitName]; ok {
				if changed.SubState != "running" {
					fmt.Fprintf(w, "Build completed\n", changed.SubState)
					break wait
				}
			}
		case err := <-errch:
			fmt.Fprintf(w, "Error %+v\n", err)
		case <-time.After(10 * time.Second):
			log.Print("job_build_image:", "timeout")
			break wait
		}
	}

	stdout.Close()
	select {
	case erri := <-ioerr:
		log.Printf("job_build_image: Error from IO on wait: %+v", erri)
	case <-time.After(15 * time.Second):
		log.Printf("job_build_image: Timeout waiting for write to complete")
	}
}
