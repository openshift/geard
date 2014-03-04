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

type InstallContainerRequest struct {
	RequestIdentifier

	Id    gears.Identifier
	Image string

	// A simple container is allowed to default to normal Docker
	// options like -P.  If simple is true no user or home
	// directory is created and SSH is not available
	Simple bool
	// Should this container be run in an isolated fashion
	// (separate user, permission changes)
	Isolate bool
	// Should this container be run in a socket activated fashion
	// Implies Isolated (separate user, permission changes,
	// no port forwarding, socket activated).
	// If UseSocketProxy then socket files are proxies to the
	// appropriate port
	SocketActivation bool
	SkipSocketProxy  bool

	Ports        gears.PortPairs
	Environment  *ExtendedEnvironmentData
	NetworkLinks *gears.NetworkLinks

	// Should the container be started by default
	Started bool
}

func (req *InstallContainerRequest) Check() error {
	if req.SocketActivation && len(req.Ports) == 0 {
		req.SocketActivation = false
		req.Isolate = true
	}
	if req.Image == "" {
		return errors.New("A container must have an image identifier")
	}
	if req.Environment != nil {
		if req.Environment.Id == gears.InvalidIdentifier {
			return errors.New("You must specify an environment identifier on creation.")
		}
		if err := req.Environment.Check(); err != nil {
			return err
		}
	}
	if req.NetworkLinks != nil {
		if err := req.NetworkLinks.Check(); err != nil {
			return err
		}
	}
	if req.Ports == nil {
		req.Ports = make([]gears.PortPair, 0)
	}
	return nil
}

func dockerPortSpec(p gears.PortPairs) string {
	var portSpec bytes.Buffer
	for i := range p {
		portSpec.WriteString(fmt.Sprintf("-p %d:%d ", p[i].External, p[i].Internal))
	}
	return portSpec.String()
}

func (req *InstallContainerRequest) Execute(resp JobResponse) {

	id := req.Id
	unitName := id.UnitNameFor()
	unitPath := id.UnitPathFor()
	unitDefinitionPath := id.UnitDefinitionPathFor()
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
		if err := env.Fetch(); err != nil {
			resp.Failure(ErrGearCreateFailed)
			return
		}
	}

	// open and lock the base path (to prevent simultaneous updates)
	state, exists, err := utils.OpenFileExclusive(unitPath, 0660)
	if err != nil {
		log.Print("install_container: Unable to open unit file: ", err)
		resp.Failure(ErrGearCreateFailed)
	}
	defer state.Close()

	// write a new file to disk that describes the new service
	unit, err := utils.CreateFileExclusive(unitVersionPath, 0660)
	if err != nil {
		log.Print("install_container: Unable to open unit file: ", err)
		resp.Failure(ErrGearCreateFailed)
		return
	}
	defer unit.Close()

	// if this is an existing container, read the currently reserved ports
	existingPorts := gears.PortPairs{}
	if exists {
		existingPorts, err = gears.GetExistingPorts(id)
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				log.Print("install_container: Unable to read existing ports from file: ", err)
				resp.Failure(ErrGearCreateFailed)
				return
			}
		}
	}

	// allocate and reserve ports for this gear
	reserved, erra := gears.AtomicReserveExternalPorts(unitVersionPath, req.Ports, existingPorts)
	if erra != nil {
		log.Printf("install_container: Unable to reserve external ports: %+v", erra)
		resp.Failure(ErrGearCreateFailed)
		return
	}
	if len(reserved) > 0 {
		resp.WritePendingSuccess("PortMapping", reserved)
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
			resp.Failure(ErrGearCreateFailed)
			return
		}
		environmentPath = env.Id.EnvironmentPathFor()
	}

	// write the network links (if any) to disk
	if req.NetworkLinks != nil {
		if errw := req.NetworkLinks.Write(id.NetworkLinksPathFor(), false); errw != nil {
			resp.Failure(ErrGearCreateFailed)
			return
		}
	}

	slice := "gear-small"

	// write the definition unit file
	args := gears.ContainerUnit{
		Gear:     id,
		Image:    req.Image,
		PortSpec: portSpec,
		Slice:    slice + ".slice",

		Isolate: req.Isolate,

		ReqId: req.RequestIdentifier.String(),

		HomeDir:         id.HomePath(),
		EnvironmentPath: environmentPath,
		ExecutablePath:  filepath.Join(config.GearBasePath(), "bin", "gear"),
		IncludePath:     "",

		PortPairs:            reserved,
		SocketUnitName:       socketUnitName,
		SocketActivationType: socketActivationType,
	}

	var unitTemplate *template.Template
	switch {
	case req.Simple:
		unitTemplate = gears.SimpleContainerUnitTemplate
	case req.SocketActivation:
		unitTemplate = gears.ContainerSocketActivatedUnitTemplate
	default:
		unitTemplate = gears.ContainerUnitTemplate
	}

	if erre := unitTemplate.Execute(unit, args); erre != nil {
		log.Printf("install_container: Unable to output template: %+v", erre)
		resp.Failure(ErrGearCreateFailed)
		defer os.Remove(unitVersionPath)
		return
	}
	if err := unit.Close(); err != nil {
		log.Printf("install_container: Unable to finish writing unit: %+v", err)
		resp.Failure(ErrGearCreateFailed)
		defer os.Remove(unitVersionPath)
		return
	}

	// swap the new definition with the old one
	if err := utils.AtomicReplaceLink(unitVersionPath, unitDefinitionPath); err != nil {
		log.Printf("install_container: Failed to activate new unit: %+v", err)
		resp.Failure(ErrGearCreateFailed)
		return
	}

	// write the gear state (active, or not active) based on the current start
	// state
	if errs := gears.WriteGearStateTo(state, id, req.Started); errs != nil {
		log.Print("install_container: Unable to write state file: ", err)
		resp.Failure(ErrGearCreateFailed)
		return
	}
	if err := state.Close(); err != nil {
		log.Print("install_container: Unable to close state file: ", err)
		resp.Failure(ErrGearCreateFailed)
		return
	}

	// Generate the socket file and ignore failures
	paths := []string{unitPath}
	if req.SocketActivation {
		if err := writeSocketUnit(socketUnitPath, &args); err == nil {
			paths = []string{unitPath, socketUnitPath}
		}
	}

	if err := systemd.EnableAndReloadUnit(systemd.Connection(), unitName, paths...); err != nil {
		log.Printf("install_container: Could not enable gear %s: %v", unitName, err)
		resp.Failure(ErrGearCreateFailed)
		return
	}

	if req.Started {
		if req.SocketActivation {
			// Start the socket file, not the service and ignore failures
			if err := systemd.Connection().StartUnitJob(socketUnitName, "fail"); err != nil {
				log.Printf("install_container: Could not start gear socket %s: %v", socketUnitName, err)
				resp.Failure(ErrGearCreateFailed)
				return
			}
		} else {
			if err := systemd.Connection().StartUnitJob(unitName, "fail"); err != nil {
				log.Printf("install_container: Could not start gear %s: %v", unitName, err)
				resp.Failure(ErrGearCreateFailed)
				return
			}

		}
	}

	w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
	if req.Started {
		fmt.Fprintf(w, "Gear %s is starting\n", id)
	} else {
		fmt.Fprintf(w, "Gear %s is installed\n", id)
	}
}

func writeSocketUnit(path string, args *gears.ContainerUnit) error {
	socketUnit, err := os.Create(path)
	if err != nil {
		log.Print("install_container: Unable to open socket file: ", err)
		return err
	}
	defer socketUnit.Close()

	socketTemplate := gears.ContainerSocketTemplate
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

func (j *InstallContainerRequest) Join(job Job, complete <-chan bool) (joined bool, done <-chan bool, err error) {
	if old, ok := job.(*InstallContainerRequest); !ok {
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
