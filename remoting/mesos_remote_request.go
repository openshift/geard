// +build mesos

package remoting

import (
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/mesos"
	mesosjobs "github.com/openshift/geard/mesos/jobs"

	"log"
)

type RemoteLocator mesos.RemoteLocator
type RemoteExecutable mesos.RemoteExecutable

func NewDispatcher(locator mesos.RemoteLocator, logger *log.Logger) *mesos.MesosDispatcher {
	return mesos.NewMesosDispatcher(locator, logger)
}

func IsRemoteJob(job jobs.Job) (remotable mesos.RemoteExecutable, isRemote bool) {
	remotable, isRemote = job.(mesos.RemoteExecutable)
	return
}

func InstallContainerRequestFor(job jobs.InstallContainerRequest) *mesosjobs.MesosInstallContainerRequest {
	return &mesosjobs.MesosInstallContainerRequest{InstallContainerRequest: job}
}

func DeleteContainerRequestFor(job jobs.DeleteContainerRequest, label string) *mesosjobs.MesosDeleteContainerRequest {
	return &mesosjobs.MesosDeleteContainerRequest{DeleteContainerRequest: job, Label: label}
}
