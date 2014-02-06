package geard

import (
	//"errors"
	"fmt"
	"io"
)

type contentJobRequest struct {
	Request jobRequest
	Type    string
	Locator string
	Subpath string
	Output  io.Writer
}

func (j *contentJobRequest) Id() RequestIdentifier {
	return j.Request.RequestId
}
func (j *contentJobRequest) Fast() bool {
	return true
}

func (j *contentJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Yo, I did your content job %+v\n", j)
}

//
// A content retrieval job cannot be joined, and so should continue (we allow multiple inflight CR)
//
func (j *contentJobRequest) Join(job Job, complete <-chan bool) (bool, <-chan bool, error) {
	return false, nil, nil
}
