// Container jobs control container related actions on a server.  Each request
// object has a default implementation on Linux via systemd, and a structured
// response if necessary.  The Execute() method is separated so that client code
// and server code can share common sanity checks.
package jobs

import (
	"errors"
	"net/url"

	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/port"
)

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
	jobs.RequestIdentifier `json:"-"`

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
	NetworkLinks *containers.NetworkLinks

	// Should the container be started by default
	Started bool

	// name of systemd slice unit to associate with container
	SystemdSlice string
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

const PendingPortMappingName = "PortMapping"

func (j *InstallContainerRequest) PortMappingsFrom(pending map[string]interface{}) (port.PortPairs, bool) {
	p, ok := pending[PendingPortMappingName].(port.PortPairs)
	return p, ok
}

type StartedContainerStateRequest struct {
	Id containers.Identifier
}

type StoppedContainerStateRequest struct {
	Id containers.Identifier
}

type RestartContainerRequest struct {
	Id containers.Identifier
}

type BuildImageRequest struct {
	Name         string
	Source       string
	Tag          string
	BaseImage    string
	RuntimeImage string
	Clean        bool
	Verbose      bool
	CallbackUrl  string
}

func (e *BuildImageRequest) Check() error {
	if e.Name == "" {
		return errors.New("An identifier must be specified for this build")
	}
	if e.BaseImage == "" {
		return errors.New("A base image is required to start a build")
	}
	if e.Source == "" {
		return errors.New("A source input is required to start a build")
	}
	if e.CallbackUrl != "" {
		_, err := url.ParseRequestURI(e.CallbackUrl)
		if err != nil {
			return errors.New("The callbackUrl was an invalid URL")
		}
	}
	return nil
}

type ContainerLogRequest struct {
	Id containers.Identifier
}

type ContainerPortsRequest struct {
	Id containers.Identifier
}

type ContainerPortsResponse struct {
	Ports port.PortPairs
}

type ContainerStatusRequest struct {
	Id containers.Identifier
}

const ContentTypeEnvironment = "env"

type ContentRequest struct {
	Type    string
	Locator string
	Subpath string
}

type DeleteContainerRequest struct {
	Id containers.Identifier
}

type PutEnvironmentRequest struct {
	containers.EnvironmentDescription
}

type PatchEnvironmentRequest struct {
	containers.EnvironmentDescription
}

type LinkContainersRequest struct {
	*containers.ContainerLinks
}

type ListImagesRequest struct {
	DockerSocket string
}

type ListContainersRequest struct {
}

type UnitResponse struct {
	Id          string
	ActiveState string
	SubState    string
}
type UnitResponses []UnitResponse

type ContainerUnitResponse struct {
	UnitResponse
	LoadState string
	JobType   string `json:"JobType,omitempty"`
	// Used by consumers
	Server string `json:"Server,omitempty"`
}
type ContainerUnitResponses []ContainerUnitResponse

type ListContainersResponse struct {
	Containers ContainerUnitResponses
}

type ListBuildsRequest struct{}
type ListBuildsResponse struct {
	Builds UnitResponses
}

type PurgeContainersRequest struct{}

type RunContainerRequest struct {
	Name      string
	Image     string
	Command   string
	Arguments []string
}

func (e *RunContainerRequest) Check() error {
	if e.Name == "" {
		return errors.New("A name must be specified for this container execution")
	}
	if e.Image == "" {
		return errors.New("An image must be specified for this container execution")
	}
	return nil
}
