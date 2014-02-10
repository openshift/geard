package geard

import (
	"errors"
	//"fmt"
	"io"
	//"io/ioutil"
	//"time"
	"bytes"
	"fmt"
	"log"
	"os"
)

type createContainerJobRequest struct {
	jobRequest
	GearId Identifier
	UserId string
	Image  string
	Output io.Writer
	Data   *extendedCreateContainerData
}

type PortPair struct {
	Internal int
	External int
}

type extendedCreateContainerData struct {
	Ports [](PortPair)
}

func (j *createContainerJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Creating gear %s ... \n", j.GearId)

	unitPath := j.GearId.UnitPathFor()

	unit, err := os.OpenFile(unitPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if os.IsExist(err) {
		fmt.Fprintf(j.Output, "A container already exists for this gear")
		return
	} else if err != nil {
		log.Print("job_create_container: Unable to create unit file: ", err)
		fmt.Fprintf(j.Output, "Unable to create a gear for this container due to %s\n", err.Error())
		return
	}

	var portSpec bytes.Buffer
	if len(j.Data.Ports) > 0 {
		portSpec.WriteString("-p ")
		for _, ports := range j.Data.Ports {
			portSpec.WriteString(fmt.Sprintf(" %d:%d", ports.External, ports.Internal))
		}
	}

	containerUnitTemplate.Execute(unit, containerUnit{j.GearId, j.Image, portSpec.String()})
	fmt.Fprintf(unit, "\n\n# Gear information\nX-GearId=%s\nX-ContainerImage=%s\nX-ContainerUserId=%s\nX-ContainerRequestId=%s\n", j.GearId, j.Image, j.UserId, j.Id().ToShortName())
	unit.Close()

	fmt.Fprintf(j.Output, "Unit in place %s ... \n", j.GearId)
	if _, _, err := SystemdConnection().EnableUnitFiles([]string{unitPath}, false, false); err != nil {
		log.Printf("job_create_container: Failed enabling %s: %s", unitPath, err.Error())
		fmt.Fprintf(j.Output, "Unable to enable gear for this container due to %s\n", err.Error())
		return
	}

	stdout, err := ProcessLogsFor(j.GearId)
	if err != nil {
		stdout = emptyReader
		fmt.Fprintf(j.Output, "Unable to fetch journal logs: %s\n", err.Error())
	}
	defer stdout.Close()
	go io.Copy(j.Output, stdout)

	unitName := j.GearId.UnitNameFor()
	status, err := SystemdConnection().StartUnit(unitName, "fail")
	if err != nil {
		fmt.Fprintf(j.Output, "Could not start gear %s\n", err.Error())
	} else if status != "done" {
		fmt.Fprintf(j.Output, "Gear did not start successfully: %s\n", status)
	} else {
		fmt.Fprintf(j.Output, "Gear %s is starting\n", j.GearId)
	}

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
