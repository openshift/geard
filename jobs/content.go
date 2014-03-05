package jobs

import (
	"fmt"
	"github.com/smarterclayton/geard/containers"
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

func (j *ContentRequest) Execute(resp JobResponse) {
	switch j.Type {
	case ContentTypeEnvironment:
		id, errr := containers.NewIdentifier(j.Locator)
		if errr != nil {
			resp.Failure(SimpleJobError{JobResponseInvalidRequest, fmt.Sprintf("Invalid environment identifier: %s", errr.Error())})
			return
		}
		file, erro := os.OpenFile(id.EnvironmentPathFor(), os.O_RDONLY, 0660)
		if erro != nil {
			resp.Failure(SimpleJobError{JobResponseNotFound, fmt.Sprintf("Invalid environment: %s", erro.Error())})
			return
		}
		w := resp.SuccessWithWrite(JobResponseOk, false, false)
		if _, err := io.Copy(w, file); err != nil {
			log.Printf("job_content: Unable to write environment file: %+v", err)
		}
	}
}

//
// A content retrieval job cannot be joined, and so should continue (we allow multiple inflight CR)
//
func (j *ContentRequest) Join(job Job, complete <-chan bool) (bool, <-chan bool, error) {
	return false, nil, nil
}
