package geard

import (
	"errors"
	//"fmt"
	"encoding/json"
	"io"
	"io/ioutil"
	//"time"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type createContainerJobRequest struct {
	Request     jobRequest
	ContainerId string
	UserId      string
	Image       string
	Output      io.Writer
	Data        *extendedCreateContainerData
}

type PortPair struct {
	Internal int
	External int
}

type extendedCreateContainerData struct {
	Ports [](PortPair)
}

func NewCreateContainerJob(reqid RequestIdentifier, id string, userId string, image string, input io.Reader, output io.Writer) (Job, error) {
	if reqid == nil {
		return nil, errors.New("All jobs must define a request id")
	}
	if id == "" {
		return nil, errors.New("A container must have an identifier")
	}
	if userId == "" {
		return nil, errors.New("A container must have a user identifier")
	}
	if image == "" {
		return nil, errors.New("A container must have an image locator")
	}
	if input == nil {
		input = emptyReader
	}
	dec := json.NewDecoder(input)
	data := extendedCreateContainerData{}
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	if data.Ports == nil {
		data.Ports = make([]PortPair, 0)
	}

	if output == nil {
		output = ioutil.Discard
	}
	return &createContainerJobRequest{jobRequest{reqid}, id, userId, image, output, &data}, nil
}

func (j *createContainerJobRequest) Id() RequestIdentifier {
	return j.Request.RequestId
}
func (j *createContainerJobRequest) Fast() bool {
	return false
}

func (j *createContainerJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Creating gear %s ... \n", j.ContainerId)

	unitPath := PathForContainerUnit(j.ContainerId)
	unitName := filepath.Base(unitPath)
	unit, err := os.OpenFile(unitPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err == os.ErrExist {
		fmt.Fprintf(j.Output, "A container already exists for this gear")
		return
	} else if err != nil {
		log.Print("job_create_container: Unable to create unit file: ", err)
		fmt.Fprintf(j.Output, "Unable to create a gear for this container due to %s", err.Error())
		return
	}

	var portSpec bytes.Buffer
	if len(j.Data.Ports) > 0 {
		portSpec.WriteString("-p ")
		for _, ports := range j.Data.Ports {
			portSpec.WriteString(fmt.Sprintf(" %d:%d", ports.External, ports.Internal))
		}
	}

	containerUnitTemplate.Execute(unit, containerUnit{j.ContainerId, j.Image, portSpec.String()})
	fmt.Fprintf(unit, "\n\n[Gear]\nx-ContainerId=%s\nx-ContainerImage=%s\nx-ContainerUserId=%s\nx-ContainerRequestId=%s\n", j.ContainerId, j.Image, j.UserId, j.Id().ToHex())
	unit.Close()
	fmt.Fprintf(j.Output, "Unit in place %s ... \n", j.ContainerId)
	if _, _, err := SystemdConnection().EnableUnitFiles([]string{unitPath}, false, false); err != nil {
		log.Printf("job_create_container: Failed enabling %s: %s", unitPath, err.Error())
		fmt.Fprintf(j.Output, "Unable to create a gear for this container due to %s", err.Error())
		return
	}

	cmd := exec.Command("/usr/bin/journalctl", "--since=now", "-f", "--unit", unitName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdout = emptyReader
		fmt.Fprintf(j.Output, "Unable to fetch journal logs: %s", err.Error())
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(j.Output, "Unable to start journal logging: %s", err.Error())
	} else if stdout != nil {
		go io.Copy(j.Output, stdout)
	}

	status, err := SystemdConnection().StartUnit(unitName, "fail")
	if err != nil {
		fmt.Fprintf(j.Output, "Could not start gear %s", err.Error())
	} else if status != "done" {
		fmt.Fprintf(j.Output, "Gear did not start successfully: %s", err.Error())
	}
	fmt.Fprintf(j.Output, "\nGear %s is started", j.ContainerId)
	stdout.Close()
}

func (j *createContainerJobRequest) Join(job Job, complete <-chan bool) (joined bool, done <-chan bool, err error) {
	if old, ok := job.(*createContainerJobRequest); !ok {
		if old == nil {
			err = ErrRanToCompletion
		} else {
			err = errors.New("Cannot join two jobs of different types.")
		}
		return
	}

	c := make(chan bool)
	done = c
	go func() {
		close(c)
	}()
	joined = true
	return
}
