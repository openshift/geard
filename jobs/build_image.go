package jobs

import (
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/systemd"
	"github.com/smarterclayton/geard/utils"
	"github.com/smarterclayton/go-systemd/dbus"
	"io"
	"log"
	"reflect"
	"time"
)

type BuildImageRequest struct {
	*ExtendedBuildImageData
}

type ExtendedBuildImageData struct {
	Name         string
	Source       string
	Tag          string
	BaseImage    string
	RuntimeImage string
	Clean        bool
	Verbose      bool
}

func (e *ExtendedBuildImageData) Check() error {
	if e.Name == "" {
		return errors.New("An identifier must be specified for this build")
	}
	if e.BaseImage == "" {
		return errors.New("A base image is required to start a build")
	}
	if e.Source == "" {
		return errors.New("A source input is required to start a build")
	}
	return nil
}

const buildImage = "pmorie/sti-builder-go"

func (j *BuildImageRequest) Execute(resp JobResponse) {
	w := resp.SuccessWithWrite(JobResponseAccepted, true, false)

	fmt.Fprintf(w, "Processing build-image request:\n")
	// TODO: download source, add bind-mount

	unitName := containers.JobIdentifier(j.Name).UnitNameForBuild()
	unitDescription := fmt.Sprintf("Builder for %s", j.Tag)

	stdout, err := systemd.ProcessLogsForUnit(unitName)
	if err != nil {
		stdout = utils.EmptyReader
		log.Printf("job_build_image: Unable to fetch build logs: %s, %+v", err.Error(), err)
	}
	defer stdout.Close()

	conn, errc := systemd.NewConnection()
	if errc != nil {
		log.Print("job_build_image:", errc)
		fmt.Fprintf(w, "Unable to watch start status", errc)
		return
	}

	if err := conn.Subscribe(); err != nil {
		log.Print("job_build_image:", err)
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
	log.Printf("build_image: Running build %s", unitName)

	startCmd := []string{
		"/usr/bin/docker", "run",
		"-rm",
		"-v", "/run/docker.sock:/run/docker.sock",
		"-t", buildImage,
		"sti", "build", j.Source, j.BaseImage, j.Tag,
		"-U", "unix:///run/docker.sock",
	}
	log.Printf("build_image: Will execute %v", startCmd)

	if j.RuntimeImage != "" {
		startCmd = append(startCmd, "--runtime-image")
		startCmd = append(startCmd, j.RuntimeImage)
	}

	if j.Clean {
		startCmd = append(startCmd, "--clean")
	}

	if j.Verbose {
		startCmd = append(startCmd, "--debug")
	}

	status, err := systemd.Connection().StartTransientUnit(
		unitName,
		"fail",
		dbus.PropExecStart(startCmd, true),
		dbus.PropDescription(unitDescription),
		dbus.PropRemainAfterExit(true),
		dbus.PropSlice("container.slice"),
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

	go io.Copy(w, stdout)

wait:
	for {
		select {
		case c := <-changes:
			if changed, ok := c[unitName]; ok {
				if changed.SubState != "running" {
					fmt.Fprintf(w, "Build completed\n")
					break wait
				}
			}
		case err := <-errch:
			fmt.Fprintf(w, "Error %+v\n", err)
		case <-time.After(25 * time.Second):
			log.Print("job_build_image:", "timeout")
			break wait
		}
	}

	stdout.Close()
}
