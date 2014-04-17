package jobs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/geard/utils"
	"github.com/openshift/go-systemd/dbus"
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
	CallbackUrl  string
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

const (
	buildImage     = "pmorie/sti-builder-go"
	gearBinaryPath = "/usr/bin/gear"
)

func (j *BuildImageRequest) Execute(resp JobResponse) {
	log.Println("BuildImageRequest: Execute start")

	if j.CallbackUrl == "" {
		// if no callback is specified, we tail the log in the response
		j.ExecuteInternal(resp)
	} else {
		// if callback is specified, we return immediately and schedule thread to complete task
		resp.Success(JobResponseAccepted)
		// process the build in another thread
		go j.ExecuteInternal(resp)
	}

	log.Println("BuildImageRequest: Execute end")
}

func (j *BuildImageRequest) ExecuteCallback(buf *bytes.Buffer) {
	log.Printf("BuildImageRequest: ExecuteCallback start")

	if j.CallbackUrl != "" {

		// log results to console
		log.Printf("BuildImageRequest: ExecuteCallback received notification")
		log.Printf("BuildImageRequest: Build Log:\n%s", buf.String())
		log.Printf("BuildImageRequest: ExecuteCallback via POST to url: %s", j.CallbackUrl)

		// callback creates a json model
		payloadBuffer := new(bytes.Buffer)
		bufferedWriter := bufio.NewWriter(payloadBuffer)
		jsonWriter := json.NewEncoder(bufferedWriter)
		d := map[string]string{"payload": buf.String()}
		jsonWriter.Encode(d)
		bufferedWriter.Flush()

		log.Printf("BuildImageRequest: ExecuteCallback JSON:\n%s", payloadBuffer)

		// send log to callback
		resp, err := http.Post(j.CallbackUrl, "application/json", payloadBuffer)

		// TODO determine any pertinent retry behavior
		if err != nil {
			log.Printf("BuildImageRequest: ExecuteCallback error: %s", err)
		}
		if resp != nil {
			if resp.StatusCode > 400 {
				log.Printf("BuildImageRequest: ExecuteCallback callback failed")
			} else {
				log.Printf("BuildImageRequest: ExecuteCallback callback completed")
			}
		}
	}

	log.Printf("BuildImageRequest: ExecuteCallback end")
}

func (j *BuildImageRequest) ExecuteInternal(resp JobResponse) {

	log.Println("BuildImageRequest: ExecuteInternal start")

	// if no callback is specified, we tail the log in the response
	var w io.Writer
	var buf *bytes.Buffer
	var internalWriter *bufio.Writer
	if j.CallbackUrl == "" {
		w = resp.SuccessWithWrite(JobResponseAccepted, true, false)
	} else {
		buf = new(bytes.Buffer)
		internalWriter = bufio.NewWriter(buf)
		w = internalWriter
	}

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

	var startCmd []string

	if _, err := os.Stat(gearBinaryPath); err != nil {
		log.Println("gear executable is not installed on system; using sti builder image")
		startCmd = []string{
			"/usr/bin/docker", "run",
			"-rm",
			"-v", "/run/docker.sock:/run/docker.sock",
			"-t", buildImage,
			"sti", "build", j.Source, j.BaseImage, j.Tag,
			"-U", "unix:///run/docker.sock",
		}
	} else {
		startCmd = []string{
			gearBinaryPath, "build", j.Source, j.BaseImage, j.Tag,
		}
	}

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

	log.Printf("build_image: Will execute %v", startCmd)
	status, err := systemd.Connection().StartTransientUnit(
		unitName,
		"fail",
		dbus.PropExecStart(startCmd, true),
		dbus.PropDescription(unitDescription),
		dbus.PropRemainAfterExit(true),
		dbus.PropSlice("container-small.slice"),
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
		case change := <-changes:
			if changed, ok := change[unitName]; ok {
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

	// send an operation complete message to any listeners on channel
	if j.CallbackUrl != "" {
		internalWriter.Flush()
		j.ExecuteCallback(buf)
	}

	log.Println("BuildImageRequest: ExecuteInternal end")
}
