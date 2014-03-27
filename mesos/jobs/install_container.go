// +build mesos

package jobs

import (
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/mesos"

	"encoding/json"
)

type MesosInstallContainerRequest struct {
	mesos.DefaultRequest
	InstallContainerRequest jobs.InstallContainerRequest
}

//Run on Scheduler
func (r *MesosInstallContainerRequest) RequestName() string {
	return "Install container request: " + string(r.InstallContainerRequest.Id)
}

func (r *MesosInstallContainerRequest) Method() string {
	return "MesosInstallContainerRequest"
}

func (r *MesosInstallContainerRequest) MarshalRequestBody() (data []byte, err error) {
	data, err = json.Marshal(r.InstallContainerRequest)
	return
}

//Run on Executor
func (r *MesosInstallContainerRequest) UnmarshalRequestBody(data []byte, id jobs.RequestIdentifier) (err error) {
	r.InstallContainerRequest = jobs.InstallContainerRequest{}
	err = json.Unmarshal(data, &r.InstallContainerRequest)
	r.InstallContainerRequest.RequestIdentifier = id
	return
}

func (r *MesosInstallContainerRequest) Execute(response jobs.JobResponse) {
	r.InstallContainerRequest.Execute(response)
}
