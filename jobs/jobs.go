package jobs

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

type Job interface {
	Execute(JobResponse)
}

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
type JobResponse interface {
	StreamResult() bool

	Success(t JobResponseSuccess)
	SuccessWithData(t JobResponseSuccess, data interface{})
	SuccessWithWrite(t JobResponseSuccess, flush, structured bool) io.Writer
	Failure(reason JobError)

	WriteClosed() <-chan bool
	WritePendingSuccess(name string, value interface{})
}

type JobResponseSuccess int
type JobResponseFailure int

// A structured error response for a job.
type JobError interface {
	error
	ResponseFailure() JobResponseFailure
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
