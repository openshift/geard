// A job is a unit of execution that is abstracted from its environment.
package jobs

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

// A job is a unit of work - it may execute and return structured
// data or stream a response.
type Job interface {
	Execute(Response)
}

// Convenience wrapper for an anonymous function.
type JobFunction func(Response)

func (job JobFunction) Execute(res Response) {
	job(res)
}

// A client may rejoin a running job by re-executing the request,
// and a job that supports this interface will be notified that
// a second client has connected.  Typically the join will stream
// output or return the result of the call, but not do any actual
// work.
type Join interface {
	Join(Job, <-chan bool) (bool, <-chan bool, error)
}

type LabeledJob interface {
	JobLabel() string
}

// A job may return a structured error, a stream of unstructured data,
// or a stream of structured data.  In general, jobs only stream on
// success - a failure is written immediately.  A streaming job
// may write speculative side channel data that will be returned when
// a successful response occurs, or thrown away when an error is written.
// Error writes are final
type Response interface {
	StreamResult() bool

	Success(t ResponseSuccess)
	SuccessWithData(t ResponseSuccess, data interface{})
	SuccessWithWrite(t ResponseSuccess, flush, structured bool) io.Writer
	Failure(reason error)

	WritePendingSuccess(name string, value interface{})
}

type ResponseSuccess int
type ResponseFailure int

// A structured error response for a job.
type JobError interface {
	error
	ResponseFailure() ResponseFailure
	ResponseData() interface{} // May be nil if no data is returned to a client
}

type JobContext struct {
	Id   RequestIdentifier
	User string
}

type RequestIdentifier []byte

func (r RequestIdentifier) String() string {
	return strings.Trim(r.Exact(), "=")
}

func (r RequestIdentifier) Exact() string {
	return base64.URLEncoding.EncodeToString(r)
}

func NewRequestIdentifier() RequestIdentifier {
	i := make(RequestIdentifier, 16)
	rand.Read(i)
	return i
}

func NewRequestIdentifierFromString(s string) (RequestIdentifier, error) {
	var raw []byte
	switch len(s) {
	case 32:
		b, err := hex.DecodeString(s)
		if err != nil {
			return nil, err
		}
		raw = b
	case 22:
		s = s + "=="
		fallthrough
	case 24:
		b, err := base64.URLEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
		raw = b
	default:
		return nil, errors.New("Request ID must be 22 base64 characters or 32 hexadecimal characters.")
	}
	return RequestIdentifier(raw), nil
}
