// +build linux

package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	csystemd "github.com/openshift/geard/containers/systemd"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/geard/utils"
)

func dockerPortSpec(p port.PortPairs) string {
	var portSpec bytes.Buffer
	for i := range p {
		portSpec.WriteString(fmt.Sprintf("-p %d:%d ", p[i].External, p[i].Internal))
	}
	return portSpec.String()
}

func (req *InstallContainerRequest) Execute(resp jobs.Response) {
	id := req.Id
	unitName := id.UnitNameFor()
	unitPath := id.UnitPathFor()
	unitVersionPath := id.VersionedUnitPathFor(req.RequestIdentifier.String())

	socketUnitName := id.SocketUnitNameFor()
	socketUnitPath := id.SocketUnitPathFor()
	var socketActivationType string
	if req.SocketActivation {
		socketActivationType = "enabled"
		if !req.SkipSocketProxy {
			socketActivationType = "proxied"
		}
	}

	// attempt to download the environment if it is remote
	env := req.Environment
	if env != nil {
		if err := env.Fetch(100 * 1024); err != nil {
			resp.Failure(ErrContainerCreateFailed)
			return
		}
		if env.Empty() {
			env = nil
		}
	}

	// open and lock the base path (to prevent simultaneous updates)
	state, exists, err := utils.OpenFileExclusive(unitPath, 0664)
	if err != nil {
		log.Print("install_container: Unable to lock unit file: ", err)
		resp.Failure(ErrContainerCreateFailed)
	}
	defer state.Close()

	// write a new file to disk that describes the new service
	unit, err := utils.CreateFileExclusive(unitVersionPath, 0664)
	if err != nil {
		log.Print("install_container: Unable to open unit file definition: ", err)
		resp.Failure(ErrContainerCreateFailed)
		return
	}
	defer unit.Close()

	// if this is an existing container, read the currently reserved ports
	existingPorts := port.PortPairs{}
	if exists {
		existingPorts, err = containers.GetExistingPorts(id)
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				log.Print("install_container: Unable to read existing ports from file: ", err)
				resp.Failure(ErrContainerCreateFailed)
				return
			}
		}
	}

	// allocate and reserve ports for this container
	reserved, erra := portReserver.AtomicReserveExternalPorts(unitVersionPath, req.Ports, existingPorts)
	if erra != nil {
		log.Printf("install_container: Unable to reserve external ports: %+v", erra)
		resp.Failure(ErrContainerCreateFailedPortsReserved)
		return
	}
	if len(reserved) > 0 {
		resp.WritePendingSuccess(PendingPortMappingName, reserved)
	}

	var portSpec string
	if req.Simple && len(reserved) == 0 {
		portSpec = "-P"
	} else {
		portSpec = dockerPortSpec(reserved)
	}

	// write the environment to disk
	var environmentPath string
	if env != nil {
		if errw := env.Write(false); errw != nil {
			resp.Failure(ErrContainerCreateFailed)
			return
		}
		environmentPath = env.Id.EnvironmentPathFor()
	}

	// write the network links (if any) to disk
	if req.NetworkLinks != nil {
		if errw := req.NetworkLinks.Write(id.NetworkLinksPathFor(), false); errw != nil {
			resp.Failure(ErrContainerCreateFailed)
			return
		}
	}

	var sliceName string
	if "" == req.SystemdSlice {
		sliceName = DefaultSlice
	} else {
		sliceNames := []string{}
		sliceFound := false
		for _, name := range ListSliceNames() {
			sliceNames = append(sliceNames, name)
			if req.SystemdSlice == name {
				sliceFound = true
				break
			}
		}

		if sliceFound {
			sliceName = req.SystemdSlice
		} else {
			log.Printf("'%s' is not a valid systemd slice. Must be one of [%s]", req.SystemdSlice, strings.Join(sliceNames, ", "))
			resp.Failure(ErrContainerCreateFailedInvalidSlice)
			return
		}
	}

	// write the definition unit file
	args := csystemd.ContainerUnit{
		Id:       id,
		Image:    req.Image,
		PortSpec: portSpec,
		Slice:    sliceName + ".slice",

		Isolate: req.Isolate,

		ReqId: req.RequestIdentifier.String(),

		HomeDir:         id.HomePath(),
		RunDir:          id.RunPathFor(),
		EnvironmentPath: environmentPath,
		ExecutablePath:  filepath.Join("/", "usr", "bin", "gear"),
		IncludePath:     "",

		PortPairs:            reserved,
		SocketUnitName:       socketUnitName,
		SocketActivationType: socketActivationType,

		DockerFeatures: config.SystemDockerFeatures,
	}

	var templateName string
	switch {
	case req.SocketActivation:
		templateName = "SOCKETACTIVATED"
	case config.SystemDockerFeatures.ForegroundRun:
		templateName = "FOREGROUND"
	default:
		templateName = "SIMPLE"
	}

	if erre := csystemd.ContainerUnitTemplate.ExecuteTemplate(unit, templateName, args); erre != nil {
		log.Printf("install_container: Unable to output template: %+v", erre)
		resp.Failure(ErrContainerCreateFailed)
		defer os.Remove(unitVersionPath)
		return
	}
	if err := unit.Close(); err != nil {
		log.Printf("install_container: Unable to finish writing unit: %+v", err)
		resp.Failure(ErrContainerCreateFailed)
		defer os.Remove(unitVersionPath)
		return
	}

	// swap the new definition with the old one
	if err := utils.AtomicReplaceLink(unitVersionPath, unitPath); err != nil {
		log.Printf("install_container: Failed to activate new unit: %+v", err)
		resp.Failure(ErrContainerCreateFailed)
		return
	}
	state.Close()

	// write whether this container should be started on next boot
	if req.Started {
		if errs := csystemd.SetUnitStartOnBoot(id, true); errs != nil {
			log.Print("install_container: Unable to write container boot link: ", err)
			resp.Failure(ErrContainerCreateFailed)
			return
		}
	}

	// Generate the socket file and ignore failures
	paths := []string{unitPath}
	if req.SocketActivation {
		if err := writeSocketUnit(socketUnitPath, &args); err == nil {
			paths = []string{unitPath, socketUnitPath}
		}
	}

	if err := systemd.EnableAndReloadUnit(systemd.Connection(), unitName, paths...); err != nil {
		log.Printf("install_container: Could not enable container %s (%v): %v", unitName, paths, err)
		resp.Failure(ErrContainerCreateFailed)
		return
	}

	if req.Started {
		if req.SocketActivation {
			// Start the socket file, not the service and ignore failures
			if err := systemd.Connection().StartUnitJob(socketUnitName, "replace"); err != nil {
				log.Printf("install_container: Could not start container socket %s: %v", socketUnitName, err)
				resp.Failure(ErrContainerCreateFailed)
				return
			}
		} else {
			if err := systemd.Connection().StartUnitJob(unitName, "replace"); err != nil {
				log.Printf("install_container: Could not start container %s: %v", unitName, err)
				resp.Failure(ErrContainerCreateFailed)
				return
			}

		}
	}

	w := resp.SuccessWithWrite(jobs.ResponseAccepted, true, false)
	if req.Started {
		fmt.Fprintf(w, "Container %s is starting\n", id)
	} else {
		fmt.Fprintf(w, "Container %s is installed\n", id)
	}
}

func writeSocketUnit(path string, args *csystemd.ContainerUnit) error {
	socketUnit, err := os.Create(path)
	if err != nil {
		log.Print("install_container: Unable to open socket file: ", err)
		return err
	}
	defer socketUnit.Close()

	socketTemplate := csystemd.ContainerSocketTemplate
	if err := socketTemplate.Execute(socketUnit, args); err != nil {
		log.Printf("install_container: Unable to output socket template: %+v", err)
		defer os.Remove(path)
		return err
	}

	if err := socketUnit.Close(); err != nil {
		log.Printf("install_container: Unable to finish writing socket: %+v", err)
		defer os.Remove(path)
		return err
	}

	return nil
}

func (j *InstallContainerRequest) Join(job jobs.Job, complete <-chan bool) (joined bool, done <-chan bool, err error) {
	if old, ok := job.(*InstallContainerRequest); !ok {
		if old == nil {
			err = jobs.ErrRanToCompletion
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
