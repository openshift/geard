package geard

import (
	"bytes"
	"errors"
	//"fmt"
	"io"
	"io/ioutil"
	//"time"
)

type Job interface {
	Execute()

	Fast() bool
	Id() RequestIdentifier
}

type Join interface {
	Join(Job, <-chan bool) (bool, <-chan bool, error)
}

// A job may return a structured error, a stream of unstructured data,
// or a stream of structured data.  In general, jobs only stream on
// success - a failure is written immediately.  A streaming job
// may write speculative side channel data that will be returned when
// a successful response occurs, or thrown away when an error is written.
// Error writes are final
type JobResponse interface {
	StreamResult() bool

	Success(t JobResponseSuccess)
	SuccessWithData(t JobResponseSuccess, data interface{})
	SuccessWithWrite(t JobResponseSuccess, flush bool) io.Writer
	Failure(reason error)

	WriteClosed() <-chan bool
	WritePendingSuccess(name string, value interface{})
}

var (
	ErrRanToCompletion = errors.New("This job has run to completion.")
)

type JobResponseSuccess int

const (
	JobResponseOk JobResponseSuccess = iota
	JobResponseAccepted
)

type jobRequest struct {
	RequestId RequestIdentifier
}

type JobError interface {
	Failure() JobResponseFailure
}

type JobResponseFailure int

const (
	JobResponseError JobResponseFailure = iota
	JobResponseAlreadyExists
)

func (j *jobRequest) Fast() bool {
	return false
}

func (j *jobRequest) Id() RequestIdentifier {
	return j.RequestId
}

var emptyReader = ioutil.NopCloser(bytes.NewReader([]byte{}))
