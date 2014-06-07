package linux

import (
	"path/filepath"

	"github.com/openshift/geard/config"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/systemd"
)

type PortReserver interface {
	AtomicReserveExternalPorts(path string, ports, existing port.PortPairs) (port.PortPairs, error)
	ReleaseExternalPorts(ports port.PortPairs) error
}

// TODO: inject me into job implementations
var portReserver PortReserver

// Return a job extension that casts requests directly to jobs
// TODO: Move implementation out of request object and into a
//   specific package
func NewContainerExtension() jobs.JobExtension {
	return &jobs.JobInitializer{
		Extension: jobs.JobExtensionFunc(implementsJob),
		Func:      initContainers,
	}
}

func implementsJob(request interface{}) (jobs.Job, error) {
	switch r := request.(type) {
	case *cjobs.StartedContainerStateRequest:
		return &startContainer{r, systemd.Connection()}, nil
	case *cjobs.StoppedContainerStateRequest:
		return &stopContainer{r, systemd.Connection()}, nil
	case *cjobs.RestartContainerRequest:
		return &restartContainer{r, systemd.Connection()}, nil
	case *cjobs.BuildImageRequest:
		return &buildImage{r, systemd.Connection()}, nil
	case *cjobs.ContainerLogRequest:
		return &containerLog{r}, nil
	case *cjobs.ContainerPortsRequest:
		return &containerPorts{r}, nil
	case *cjobs.ContainerStatusRequest:
		return &containerStatus{r}, nil
	case *cjobs.DeleteContainerRequest:
		return &deleteContainer{r, systemd.Connection()}, nil
	case *cjobs.GetEnvironmentRequest:
		return &getEnvironment{r}, nil
	case *cjobs.PatchEnvironmentRequest:
		return &patchEnvironment{r}, nil
	case *cjobs.PutEnvironmentRequest:
		return &putEnvironent{r}, nil
	case *cjobs.InstallContainerRequest:
		return &installContainer{r, systemd.Connection()}, nil
	case *cjobs.LinkContainersRequest:
		return &linkContainers{r}, nil
	case *cjobs.ListImagesRequest:
		return &listImages{r}, nil
	case *cjobs.ListContainersRequest:
		return &listContainers{r, systemd.Connection()}, nil
	case *cjobs.ListBuildsRequest:
		return &listBuilds{r, systemd.Connection()}, nil
	case *cjobs.PurgeContainersRequest:
		return &purgeContainers{r}, nil
	case *cjobs.RunContainerRequest:
		return &runContainer{r, systemd.Connection()}, nil
	}
	return nil, jobs.ErrNoJobForRequest
}

// All container jobs depend on these invariants.
// TODO: refactor jobs to take systemd and config
//   as injected dependencies
func initContainers() error {
	if err := config.HasRequiredDirectories(); err != nil {
		return err
	}
	if err := systemd.Start(); err != nil {
		return err
	}
	if err := InitializeData(); err != nil {
		return err
	}
	allocator := port.NewPortAllocator(config.ContainerBasePath(), 4000, 60000)
	go allocator.Run()
	portReserver = &port.PortReservation{allocator}
	return nil
}

func init() {
	config.AddRequiredDirectory(
		0750,
		filepath.Join(config.ContainerBasePath(), "env", "contents"),
		filepath.Join(config.ContainerBasePath(), "ports", "descriptions"),
		filepath.Join(config.ContainerBasePath(), "ports", "interfaces"),
	)
	config.AddRequiredDirectory(
		0755,
		filepath.Join(config.SystemdBasePath(), "container-active.target.wants"),
	)
}
