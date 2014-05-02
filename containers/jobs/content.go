package jobs

import (
	"fmt"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
	"io"
	"log"
	"os"
)

const ContentTypeEnvironment = "env"

type ContentRequest struct {
	Type    string
	Locator string
	Subpath string
}

func (j *ContentRequest) Fast() bool {
	return true
}

func (j *ContentRequest) Execute(resp jobs.JobResponse) {
	switch j.Type {
	case ContentTypeEnvironment:
		id, errr := containers.NewIdentifier(j.Locator)
		if errr != nil {
			resp.Failure(jobs.SimpleJobError{jobs.JobResponseInvalidRequest, fmt.Sprintf("Invalid environment identifier: %s", errr.Error())})
			return
		}
		file, erro := os.Open(id.EnvironmentPathFor())
		if erro != nil {
			resp.Failure(ErrEnvironmentNotFound)
			return
		}
		defer file.Close()
		w := resp.SuccessWithWrite(jobs.JobResponseOk, false, false)
		if _, err := io.Copy(w, file); err != nil {
			log.Printf("job_content: Unable to write environment file: %+v", err)
			return
		}
	}
}

//
// A content retrieval job cannot be joined, and so should continue (we allow multiple inflight CR)
//
func (j *ContentRequest) Join(job jobs.Job, complete <-chan bool) (bool, <-chan bool, error) {
	return false, nil, nil
}
