package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/systemd"
	"log"
	"os"
	"strconv"
)

type CreateContainerJobRequest struct {
	JobResponse
	JobRequest
	GearId gears.Identifier
	UserId string
	Image  string
	Data   *ExtendedCreateContainerData
}

type ExtendedCreateContainerData struct {
	Ports       PortPairs
	Environment *ExtendedEnvironmentData
}

type PortPairs []gears.PortPair

func (p PortPairs) ToHeader() string {
	var pairs bytes.Buffer
	for i := range p {
		if i != 0 {
			pairs.WriteString(",")
		}
		pairs.WriteString(strconv.Itoa(int(p[i].Internal)))
		pairs.WriteString("=")
		pairs.WriteString(strconv.Itoa(int(p[i].External)))
	}
	return pairs.String()
}

func (j *CreateContainerJobRequest) Execute() {
	unitPath := j.GearId.UnitPathFor()

	env := j.Data.Environment
	if env != nil {
		if err := env.Fetch(); err != nil {
			j.Failure(ErrGearCreateFailed)
			return
		}
	}

	unit, err := os.OpenFile(unitPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if os.IsExist(err) {
		j.Failure(ErrGearAlreadyExists)
		return
	} else if err != nil {
		log.Print("job_create_container: Unable to create unit file: ", err)
		j.Failure(ErrGearCreateFailed)
		return
	}
	defer unit.Close()

	var portSpec bytes.Buffer
	if len(j.Data.Ports) > 0 {
		portSpec.WriteString("-p")
		for i := range j.Data.Ports {
			ports := &j.Data.Ports[i]
			if ports.External < 1 {
				ports.External = gears.AllocatePort()
				if ports.External == 0 {
					log.Printf("job_create_container: Unable to allocate external port for %d", ports.Internal)
					j.Failure(ErrGearCreateFailed)
					return
				}
			}
			portSpec.WriteString(fmt.Sprintf(" %d:%d", ports.External, ports.Internal))
		}

		if erra := gears.AtomicReserveExternalPorts(j.GearId.PortDescriptionPathFor(), j.Data.Ports); erra != nil {
			log.Printf("job_create_container: Unable to reserve external ports: %+v", erra)
			j.Failure(ErrGearCreateFailed)
			return
		}

		j.WritePendingSuccess("PortMapping", j.Data.Ports)
	}

	var environmentPath string
	if env != nil {
		if errw := env.Write(false); errw != nil {
			j.Failure(ErrGearCreateFailed)
			return
		}
		environmentPath = env.Id.EnvironmentPathFor()
	}

	slice := "gear-small"
	if erre := gears.ContainerUnitTemplate.Execute(unit, gears.ContainerUnit{
		j.GearId,
		j.Image,
		portSpec.String(),
		slice + ".slice",
		j.UserId,
		j.RequestId.ToShortName(),
		config.GearBasePath(),
		j.GearId.HomePath(),
		environmentPath,
	}); erre != nil {
		log.Printf("job_create_container: Unable to output template: %+v", erre)
		j.Failure(ErrGearCreateFailed)
	}
	//fmt.Fprintf(unit, "\n\n# Gear information\nX-GearId=%s\nX-ContainerImage=%s\nX-ContainerUserId=%s\nX-ContainerRequestId=%s\nX-ExposesPorts=%s\n", j.GearId, j.Image, j.UserId, j.Id().ToShortName())
	unit.Close()

	// FIXME check for j.StreamResult before attempting this
	// stdout, err := ProcessLogsFor(j.GearId)
	// if err != nil {
	// 	stdout = emptyReader
	// 	log.Printf("job_create_container: Unable to fetch journal logs: %+v", err)
	// }
	// defer stdout.Close()

	unitName := j.GearId.UnitNameFor()
	status, err := systemd.StartAndEnableUnit(systemd.SystemdConnection(), unitName, unitPath, "fail")
	if err != nil {
		log.Printf("job_create_container: Could not start gear %s: %v", unitName, err)
		j.Failure(ErrGearCreateFailed)
		return
	} else if status != "done" {
		log.Printf("job_create_container: Unit did not return 'done': %s", status)
		j.Failure(ErrGearCreateFailed)
		return
	}

	w := j.SuccessWithWrite(JobResponseAccepted, true)
	fmt.Fprintf(w, "Gear %s is starting\n", j.GearId)
	// FIXME check for j.StreamResult
	//go io.Copy(w, stdout)
	//stdout.Close()
}

func (j *CreateContainerJobRequest) Join(job Job, complete <-chan bool) (joined bool, done <-chan bool, err error) {
	if old, ok := job.(*CreateContainerJobRequest); !ok {
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
