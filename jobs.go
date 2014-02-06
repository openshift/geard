package geard

import (
	"bytes"
	"errors"
	//"fmt"
	//"io"
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

var (
	ErrRanToCompletion = errors.New("This job has run to completion.")
)

type jobRequest struct {
	RequestId RequestIdentifier
}

func (j *jobRequest) Fast() bool {
	return false
}

func (j *jobRequest) Id() RequestIdentifier {
	return j.RequestId
}

var emptyReader = ioutil.NopCloser(bytes.NewReader([]byte{}))
