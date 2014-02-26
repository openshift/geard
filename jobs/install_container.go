package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/systemd"
	"github.com/smarterclayton/geard/utils"
	"log"
	"os"
)

// Installing a Container
//
// This job will install a given container definition as a systemd service unit,
// or update the existing definition if one already exists.
//
// Preconditions for starting a gear:
//
// 1) Reserve external ports and define port mappings
// 2) Create gear user and set quota
// 3) Ensure gear volumes (persistent data) are assigned proper UID
// 4) Map the gear user to the appropriate user inside the image
// 5) Download the image locally
//
// Operations that require a started gear:
//
// 1) Set any internal iptable mappings to other gears (requires namespace)
//
// Operations that can occur after the gear is created but do not block creation:
//
// 1) Enable SSH access to the gear
//
// Operations that can occur on startup or afterwards
//
// 1) Publicly exposing ports

type InstallContainerJobRequest struct {
	JobResponse
	JobRequest
	GearId gears.Identifier
	UserId string
	Image  string
	Data   *ExtendedInstallContainerData
}

type ExtendedInstallContainerData struct {
	Ports        gears.PortPairs
	Environment  *ExtendedEnvironmentData
	NetworkLinks *gears.NetworkLinks `json:"network_links"`
}

func dockerPortSpec(p gears.PortPairs) string {
	var portSpec bytes.Buffer
	if len(p) > 0 {
		portSpec.WriteString("-p")
		for i := range p {
			portSpec.WriteString(fmt.Sprintf(" %d:%d", p[i].External, p[i].Internal))
		}
	}
	return portSpec.String()
}

func (j *InstallContainerJobRequest) Execute() {
	env := j.Data.Environment
	if env != nil {
		if err := env.Fetch(); err != nil {
			j.Failure(ErrGearCreateFailed)
			return
		}
	}

	unitPath := j.GearId.UnitPathFor()
	unitVersionPath := j.GearId.VersionedUnitPathFor(j.RequestId.String())
	unit, err := utils.CreateFileExclusive(unitVersionPath, 0666)
	if err != nil {
		log.Print("job_create_container: Unable to open unit file: ", err)
		j.Failure(ErrGearCreateFailed)
		return
	}
	defer unit.Close()

	existingPorts := gears.PortPairs{}
	if existing, err := os.OpenFile(unitPath, os.O_RDONLY, 0660); err == nil {
		existingPorts, err = gears.ReadPortsFromUnitFile(existing)
		existing.Close()
		if err != nil {
			log.Print("job_create_container: Unable to read existing ports from file: ", err)
			j.Failure(ErrGearCreateFailed)
			return
		}
	}

	reserved, erra := gears.AtomicReserveExternalPorts(unitVersionPath, j.Data.Ports, existingPorts)
	if erra != nil {
		log.Printf("job_create_container: Unable to reserve external ports: %+v", erra)
		j.Failure(ErrGearCreateFailed)
		return
	}
	if len(reserved) > 0 {
		j.WritePendingSuccess("PortMapping", reserved)
	}

	var environmentPath string
	if env != nil {
		if errw := env.Write(false); errw != nil {
			j.Failure(ErrGearCreateFailed)
			return
		}
		environmentPath = env.Id.EnvironmentPathFor()
	}

	if j.Data.NetworkLinks != nil {
		if errw := j.Data.NetworkLinks.Write(j.GearId.NetworkLinksPathFor(), false); errw != nil {
			j.Failure(ErrGearCreateFailed)
			return
		}
	}

	slice := "gear-small"
	if erre := gears.ContainerUnitTemplate.Execute(unit, gears.ContainerUnit{
		j.GearId,
		j.Image,
		dockerPortSpec(reserved),
		slice + ".slice",
		j.UserId,
		j.RequestId.String(),
		config.GearBasePath(),
		j.GearId.HomePath(),
		environmentPath,
		gears.HasBinaries(),
		gears.HasBinaries(),
	}); erre != nil {
		log.Printf("job_create_container: Unable to output template: %+v", erre)
		j.Failure(ErrGearCreateFailed)
		defer os.Remove(unitVersionPath)
		return
	}
	if errw := reserved.WritePortsToUnitFile(unit); errw != nil {
		log.Printf("job_create_container: Unable to write ports to unit: %+v", errw)
		j.Failure(ErrGearCreateFailed)
		defer os.Remove(unitVersionPath)
		return
	}
	if err := unit.Close(); err != nil {
		log.Printf("job_create_container: Unable to finish writing unit: %+v", err)
		j.Failure(ErrGearCreateFailed)
		defer os.Remove(unitVersionPath)
		return
	}

	if err := utils.AtomicReplaceLink(unitVersionPath, unitPath); err != nil {
		log.Printf("job_create_container: Failed to activate new unit: %+v", err)
		j.Failure(ErrGearCreateFailed)
		return
	}

	unitName := j.GearId.UnitNameFor()
	status, err := systemd.StartAndEnableUnit(systemd.Connection(), unitName, unitPath, "fail")
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
}

func (j *InstallContainerJobRequest) Join(job Job, complete <-chan bool) (joined bool, done <-chan bool, err error) {
	if old, ok := job.(*InstallContainerJobRequest); !ok {
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
