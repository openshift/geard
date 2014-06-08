package linux

import (
	"io"
	"log"
	"os"

	. "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
)

type getEnvironment struct {
	*GetEnvironmentRequest
}

func (j *getEnvironment) Execute(resp jobs.Response) {
	id := j.Id

	file, erro := os.Open(id.EnvironmentPathFor())
	if erro != nil {
		resp.Failure(ErrEnvironmentNotFound)
		return
	}
	defer file.Close()
	w := resp.SuccessWithWrite(jobs.ResponseOk, false, false)
	if _, err := io.Copy(w, file); err != nil {
		log.Printf("job_content: Unable to write environment file: %+v", err)
		return
	}
}

type putEnvironent struct {
	*PutEnvironmentRequest
}

func (j *putEnvironent) Execute(resp jobs.Response) {
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

type patchEnvironment struct {
	*PatchEnvironmentRequest
}

func (j *patchEnvironment) Execute(resp jobs.Response) {
	if err := j.Write(true); err != nil {
		resp.Failure(ErrEnvironmentUpdateFailed)
		return
	}
	resp.Success(jobs.ResponseOk)
}
