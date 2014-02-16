package geard

import (
	"fmt"
	"github.com/smarterclayton/geard/streams"
	"log"
)

const ContentTypeGitArchive = "gitarchive"

type contentJobRequest struct {
	JobResponse
	jobRequest
	Type    string
	Locator string
	Subpath string
}

func (j *contentJobRequest) Fast() bool {
	return j.Type != ContentTypeGitArchive
}

func (j *contentJobRequest) Execute() {
	switch j.Type {
	case ContentTypeGitArchive:
		repoId, errr := NewIdentifier(j.Locator)
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
			log.Printf("Invalid git repository stream: %v", err)
			return
		}
	}
}

//
// A content retrieval job cannot be joined, and so should continue (we allow multiple inflight CR)
//
func (j *contentJobRequest) Join(job Job, complete <-chan bool) (bool, <-chan bool, error) {
	return false, nil, nil
}
