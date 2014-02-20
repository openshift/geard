package jobs

import (
	"fmt"
	"github.com/smarterclayton/geard/streams"
	"github.com/smarterclayton/geard/gear"	
	"io"
	"log"
	"os"
)

const ContentTypeGitArchive = "gitarchive"
const ContentTypeEnvironment = "env"

type ContentJobRequest struct {
	JobResponse
	JobRequest
	Type    string
	Locator string
	Subpath string
}

func (j *ContentJobRequest) Fast() bool {
	return j.Type != ContentTypeGitArchive
}

func (j *ContentJobRequest) Execute() {
	switch j.Type {
	case ContentTypeEnvironment:
		id, errr := gear.NewIdentifier(j.Locator)
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

	case ContentTypeGitArchive:
		repoId, errr := gear.NewIdentifier(j.Locator)
		if errr != nil {
			j.Failure(SimpleJobError{JobResponseInvalidRequest, fmt.Sprintf("Invalid repository identifier: %s", errr.Error())})
			return
		}
		ref, errc := streams.NewGitCommitRef(j.Subpath)
		if errc != nil {
			j.Failure(SimpleJobError{JobResponseInvalidRequest, fmt.Sprintf("Invalid commit ref: %s", errc.Error())})
			return
		}
		w := j.SuccessWithWrite(JobResponseOk, false)
		if err := streams.WriteGitRepositoryArchive(w, repoId.RepositoryPathFor(), ref); err != nil {
			log.Printf("job_content: Invalid git repository stream: %v", err)
		}
	}
}

//
// A content retrieval job cannot be joined, and so should continue (we allow multiple inflight CR)
//
func (j *ContentJobRequest) Join(job Job, complete <-chan bool) (bool, <-chan bool, error) {
	return false, nil, nil
}
