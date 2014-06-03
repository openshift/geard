// +build linux

package jobs

import (
	"path/filepath"

	"github.com/openshift/geard/config"
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
		Extension: jobs.JobExtensionFunc(sharesImplementation),
		Func:      initContainers,
	}
}

func sharesImplementation(request interface{}) (jobs.Job, error) {
	if job, ok := request.(jobs.Job); ok {
		return job, nil
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
