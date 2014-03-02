package jobs

import (
	"fmt"
	"github.com/smarterclayton/geard/gears"
	"io"
	"log"
	"os"
)

const ContentTypeEnvironment = "env"

type ContentRequest struct {
	JobResponse
	JobRequest
	Type    string
	Locator string
	Subpath string
}

func (j *ContentRequest) Fast() bool {
	return true
}

func (j *ContentRequest) Execute() {
	switch j.Type {
	case ContentTypeEnvironment:
		id, errr := gears.NewIdentifier(j.Locator)
		if errr != nil {
			j.Failure(SimpleJobError{JobResponseInvalidRequest, fmt.Sprintf("Invalid environment identifier: %s", errr.Error())})
			return
		}
		file, erro := os.OpenFile(id.EnvironmentPathFor(), os.O_RDONLY, 0660)
		if erro != nil {
			j.Failure(SimpleJobError{JobResponseNotFound, fmt.Sprintf("Invalid environment: %s", erro.Error())})
			return
		}
		w := j.SuccessWithWrite(JobResponseOk, false)
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
