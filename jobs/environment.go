package jobs

import (
	"github.com/smarterclayton/geard/containers"
)

type PutEnvironmentRequest struct {
	*containers.EnvironmentDescription
}

func (j *PutEnvironmentRequest) Execute(resp JobResponse) {
	if err := j.Fetch(100 * 1024); err != nil {
		resp.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	if err := j.Write(false); err != nil {
		resp.Failure(ErrEnvironmentUpdateFailed)
		return
	}

	resp.Success(JobResponseOk)
}

type PatchEnvironmentRequest struct {
	*containers.EnvironmentDescription
}

func (j *PatchEnvironmentRequest) Execute(resp JobResponse) {
	if err := j.Write(true); err != nil {
		resp.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	resp.Success(JobResponseOk)
}
