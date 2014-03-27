// +build mesos

package jobs

import (
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/mesos"

	"encoding/json"
)

type MesosDeleteContainerRequest struct {
	mesos.DefaultRequest
	DeleteContainerRequest jobs.DeleteContainerRequest
	Label                  string
}

func (r *MesosDeleteContainerRequest) GetLabel() string {
	return r.Label
}

//Run on Scheduler
func (r *MesosDeleteContainerRequest) RequestName() string {
	return "Delete container request: " + string(r.DeleteContainerRequest.Id)
}

func (r *MesosDeleteContainerRequest) Method() string {
	return "MesosDeleteContainerRequest"
}

func (r *MesosDeleteContainerRequest) MarshalRequestBody() (data []byte, err error) {
	data, err = json.Marshal(r.DeleteContainerRequest)
	return
}

//Run on Executor
func (r *MesosDeleteContainerRequest) UnmarshalRequestBody(data []byte, id jobs.RequestIdentifier) (err error) {
	r.DeleteContainerRequest = jobs.DeleteContainerRequest{}
	err = json.Unmarshal(data, &r.DeleteContainerRequest)
	return
}

func (r *MesosDeleteContainerRequest) Execute(response jobs.JobResponse) {
	r.DeleteContainerRequest.Execute(response)
}
