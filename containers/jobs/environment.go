// +build linux

package jobs

import (
	"github.com/openshift/geard/jobs"
)

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

func (j *PatchEnvironmentRequest) Execute(resp jobs.Response) {
	if err := j.Write(true); err != nil {
		resp.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	resp.Success(jobs.ResponseOk)
}
