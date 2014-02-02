package geard

import (
	"errors"
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

func NewContentJob(reqid RequestIdentifier, t string, locator string, subpath string, output io.Writer) (Job, error) {
	if reqid == nil {
		return nil, errors.New("All jobs must define a request id")
	}
	if t == "" {
		return nil, errors.New("A content job must define a type")
	}
	if locator == "" {
		return nil, errors.New("A content job must define a locator")
	}
	if output == nil {
		return nil, errors.New("A content job must provide an output writer")
	}
	return &contentJobRequest{jobRequest{reqid}, t, locator, subpath, output}, nil
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
