package geard

import (
	"fmt"
	"github.com/smarterclayton/geard/streams"
	"io"
)

const ContentTypeGitArchive = "gitarchive"

type contentJobRequest struct {
	jobRequest
	Type    string
	Locator string
	Subpath string
	Output  io.Writer
}

func (j *contentJobRequest) Fast() bool {
	return j.Type != ContentTypeGitArchive
}

func (j *contentJobRequest) Execute() {
	switch j.Type {
	case ContentTypeGitArchive:
		repoId, errr := NewIdentifier(j.Locator)
		if errr != nil {
			fmt.Fprintf(j.Output, "Invalid repository identifier: %s", errr.Error())
			return
		}
		ref, errc := streams.NewGitCommitRef(j.Subpath)
		if errc != nil {
			fmt.Fprintf(j.Output, "Invalid commit ref: %s", errc.Error())
			return
		}
		if err := streams.WriteGitRepositoryArchive(j.Output, repoId.RepositoryPathFor(), ref); err != nil {
			fmt.Fprintf(j.Output, "Invalid git repository stream: %s", err.Error())
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
