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
	"path/filepath"
	"text/template"
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
	// A simple container is allowed to default to normal Docker
	// options like -P.  If simple is true no user or home
	// directory is created and SSH is not available
	Simple bool
	// Should this container be run in an isolated fashion
	// (separate user, permission changes)
	Isolate bool

	Ports        gears.PortPairs
	Environment  *ExtendedEnvironmentData
	NetworkLinks *gears.NetworkLinks `json:"network_links"`

	// Should the container be started by default
	Started bool
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
	data := j.Data
	env := data.Environment
	if env != nil {
		if err := env.Fetch(); err != nil {
			j.Failure(ErrGearCreateFailed)
			return
		}
	}

	unitName := j.GearId.UnitNameFor()
	unitPath := j.GearId.UnitPathFor()
	unitDefinitionPath := j.GearId.UnitDefinitionPathFor()
	unitVersionPath := j.GearId.VersionedUnitPathFor(j.RequestId.String())

	unit, err := utils.CreateFileExclusive(unitVersionPath, 0660)
	if err != nil {
		log.Print("job_create_container: Unable to open unit file: ", err)
		j.Failure(ErrGearCreateFailed)
		return
	}
	defer unit.Close()

	existingPorts := gears.PortPairs{}
	if existing, err := os.Open(unitPath); err == nil {
		existingPorts, err = gears.ReadPortsFromUnitFile(existing)
		existing.Close()
		if err != nil {
			log.Print("job_create_container: Unable to read existing ports from file: ", err)
			j.Failure(ErrGearCreateFailed)
			return
		}
	}

	reserved, erra := gears.AtomicReserveExternalPorts(unitVersionPath, data.Ports, existingPorts)
	if erra != nil {
		log.Printf("job_create_container: Unable to reserve external ports: %+v", erra)
		j.Failure(ErrGearCreateFailed)
		return
	}
	if len(reserved) > 0 {
		j.WritePendingSuccess("PortMapping", reserved)
	}

	var portSpec string
	if data.Simple && len(reserved) == 0 {
		portSpec = "-P"
	} else {
		portSpec = dockerPortSpec(reserved)
	}

	var environmentPath string
	if env != nil {
		if errw := env.Write(false); errw != nil {
			j.Failure(ErrGearCreateFailed)
			return
		}
		environmentPath = env.Id.EnvironmentPathFor()
	}

	if data.NetworkLinks != nil {
		if errw := data.NetworkLinks.Write(j.GearId.NetworkLinksPathFor(), false); errw != nil {
			j.Failure(ErrGearCreateFailed)
			return
		}
	}

	slice := "gear-small"
	args := gears.ContainerUnit{
		Gear:     j.GearId,
		Image:    j.Image,
		PortSpec: portSpec,
		Slice:    slice + ".slice",

		Isolate: data.Isolate,

		User:  j.UserId,
		ReqId: j.RequestId.String(),

		HomeDir:         j.GearId.HomePath(),
		EnvironmentPath: environmentPath,
		ExecutablePath:  filepath.Join(config.GearBasePath(), "bin", "gear"),
		IncludePath:     "",
	}

	var unitTemplate *template.Template
	if data.Simple {
		unitTemplate = gears.SimpleContainerUnitTemplate
	} else {
		unitTemplate = gears.ContainerUnitTemplate
	}

	if erre := unitTemplate.Execute(unit, args); erre != nil {
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

	if err := utils.AtomicReplaceLink(unitVersionPath, unitDefinitionPath); err != nil {
		log.Printf("job_create_container: Failed to activate new unit: %+v", err)
		j.Failure(ErrGearCreateFailed)
		return
	}

	if errs := gears.WriteGearState(j.GearId, data.Started); errs != nil {
		log.Print("job_create_container: Unable to write state file: ", err)
		j.Failure(ErrGearCreateFailed)
		return
	}

	if err := systemd.EnableAndReloadUnit(systemd.Connection(), unitName, unitPath); err != nil {
		log.Printf("job_create_container: Could not enable gear %s: %v", unitName, err)
		j.Failure(ErrGearCreateFailed)
		return
	}

	if data.Started {
		if err := systemd.Connection().StartUnitJob(unitName, "fail"); err != nil {
			log.Printf("job_create_container: Could not start gear %s: %v", unitName, err)
			j.Failure(ErrGearCreateFailed)
			return
		}
	}

	w := j.SuccessWithWrite(JobResponseAccepted, true)
	if data.Started {
		fmt.Fprintf(w, "Gear %s is starting\n", j.GearId)
	} else {
		fmt.Fprintf(w, "Gear %s is installed\n", j.GearId)
	}
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
