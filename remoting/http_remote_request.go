// +build !mesos

package remoting

import (
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"

	"log"
)

type RemoteLocator http.RemoteLocator
type RemoteExecutable http.RemoteExecutable

func NewDispatcher(locator http.RemoteLocator, logger *log.Logger) *http.HttpDispatcher {
	return http.NewHttpDispatcher(locator, logger)
}

func IsRemoteJob(job jobs.Job) (remotable http.RemoteExecutable, isRemote bool) {
	remotable, isRemote = job.(http.RemoteExecutable)
	return
}

func InstallContainerRequestFor(job jobs.InstallContainerRequest) *http.HttpInstallContainerRequest {
	return &http.HttpInstallContainerRequest{InstallContainerRequest: job}
}

func DeleteContainerRequestFor(job jobs.DeleteContainerRequest, label string) *http.HttpDeleteContainerRequest {
	return &http.HttpDeleteContainerRequest{DeleteContainerRequest: job, Label: label}
}
