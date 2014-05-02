package jobs

import (
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
)

type PutEnvironmentRequest struct {
	containers.EnvironmentDescription
}

func (j *PutEnvironmentRequest) Execute(resp jobs.Response) {
	if err := j.Fetch(100 * 1024); err != nil {
		resp.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	if err := j.Write(false); err != nil {
		resp.Failure(ErrEnvironmentUpdateFailed)
		return
	}

	resp.Success(jobs.ResponseOk)
}

type PatchEnvironmentRequest struct {
	containers.EnvironmentDescription
}

func (j *PatchEnvironmentRequest) Execute(resp jobs.Response) {
	if err := j.Write(true); err != nil {
		resp.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	resp.Success(jobs.ResponseOk)
}
