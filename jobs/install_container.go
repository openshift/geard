package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/geard/utils"
	"log"
	"os"
	"path/filepath"
)

var ErrContainerCreateFailedPortsReserved = SimpleJobError{JobResponseError, "Unable to create container: some ports could not be reserved."}

// Installing a Container
//
// This job will install a given container definition as a systemd service unit,
// or update the existing definition if one already exists.
//
// There are a number of run modes for containers.  Some options the caller must
// decide:
//
// * Is the container transient?
//   Should stop remove any data not in a volume - accomplished by running as a
//   specific user, and by using 'docker run --rm' as ExecStart=
//
// * Is the container isolated from the rest of the system?
//   Some use cases involve the container having access to the host disk or sockets
//   to perform system roles.  Otherwise, where possible containers should be
//   fully isolated from the host via SELinux, user namespaces, and capability
//   dropping.
//
// * Is the container hooked up to other containers?
//   The defined unit should allow regular docker linking (name based pairing),
//   the iptable-based SDN implemented here, and the propagation to the container
//   environment of that configuration (whether as ENV vars or a file).
//
// Isolated containers:
//
// An isolated container runs in a way that protects it from other containers on
// the system.  At a minimum today this means:
//
// 1) Create a user to represent the container, and run the process in the container
//    as that user.  Avoids root compromise
// 2) Assign a unique MCS category label to the container.
//
// In the future the need for #1 is removed by user namespaces, although given the
// relative immaturity of that function in the kernel at the present time it is not
// considered sufficiently secure for production use.
//
type InstallContainerRequest struct {
	RequestIdentifier `json:"-"`

	Id    containers.Identifier
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

	Ports        port.PortPairs
	Environment  *containers.EnvironmentDescription
	NetworkLinks containers.NetworkLinks

	// Should the container be started by default
	Started bool
}

func (req *InstallContainerRequest) Check() error {
	if req.SocketActivation && len(req.Ports) == 0 {
		req.SocketActivation = false
	}
	if len(req.RequestIdentifier) == 0 {
		return errors.New("A request identifier is required to create this item.")
	}
	if req.Image == "" {
		return errors.New("A container must have an image identifier")
	}
	if req.Environment != nil && !req.Environment.Empty() {
		if err := req.Environment.Check(); err != nil {
			return err
		}
		if req.Environment.Id == containers.InvalidIdentifier {
			return errors.New("You must specify an environment identifier on creation.")
		}
	}
	if req.NetworkLinks != nil {
		if err := req.NetworkLinks.Check(); err != nil {
			return err
		}
	}
	if req.Ports == nil {
		req.Ports = make([]port.PortPair, 0)
	}
	return nil
}

func dockerPortSpec(p port.PortPairs) string {
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
	reserved, erra := port.AtomicReserveExternalPorts(unitVersionPath, req.Ports, existingPorts)
	if erra != nil {
		log.Printf("install_container: Unable to reserve external ports: %+v", erra)
		resp.Failure(ErrContainerCreateFailedPortsReserved)
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

	slice := "container-small"

	// write the definition unit file
	args := containers.ContainerUnit{
		Id:       id,
		Image:    req.Image,
		PortSpec: portSpec,
		Slice:    slice + ".slice",

		Isolate: req.Isolate,

		ReqId: req.RequestIdentifier.String(),

		HomeDir:         id.HomePath(),
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

	if erre := containers.ContainerUnitTemplate.ExecuteTemplate(unit, templateName, args); erre != nil {
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
		if errs := id.SetUnitStartOnBoot(true); errs != nil {
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
			if err := systemd.Connection().StartUnitJob(socketUnitName, "fail"); err != nil {
				log.Printf("install_container: Could not start container socket %s: %v", socketUnitName, err)
				resp.Failure(ErrContainerCreateFailed)
				return
			}
		} else {
			if err := systemd.Connection().StartUnitJob(unitName, "fail"); err != nil {
				log.Printf("install_container: Could not start container %s: %v", unitName, err)
				resp.Failure(ErrContainerCreateFailed)
				return
			}

		}
	}

	w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
	if req.Started {
		fmt.Fprintf(w, "Container %s is starting\n", id)
	} else {
		fmt.Fprintf(w, "Container %s is installed\n", id)
	}
}

func writeSocketUnit(path string, args *containers.ContainerUnit) error {
	socketUnit, err := os.Create(path)
	if err != nil {
		log.Print("install_container: Unable to open socket file: ", err)
		return err
	}
	defer socketUnit.Close()

	socketTemplate := containers.ContainerSocketTemplate
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
