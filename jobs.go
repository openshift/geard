package agent

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
)

type Job interface {
}

type jobRequest struct {
	RequestId []byte
}

const (
	JobContent = iota
	JobBuild
)

var emptyReader = ioutil.NopCloser(bytes.NewReader([]byte{}))

type contentJobRequest struct {
	Request jobRequest
	Type    string
	Locator string
	Subpath string
}

func NewContentJob(reqid []byte, t string, locator string, subpath string) (Job, error) {
	if reqid == nil {
		return nil, errors.New("All jobs must define a request id")
	}
	if t == "" {
		return nil, errors.New("A content job must define a type")
	}
	if locator == "" {
		return nil, errors.New("A content job must define a locator")
	}
	return &contentJobRequest{jobRequest{reqid}, t, locator, subpath}, nil
}

type createContainerJobRequest struct {
	Request jobRequest
	Id      string
	Image   string
	Output  io.Writer
}

func NewCreateContainerJob(reqid []byte, id string, image string, input io.Reader, output io.Writer) (Job, error) {
	if reqid == nil {
		return nil, errors.New("All jobs must define a request id")
	}
	if id == "" {
		return nil, errors.New("A container must have an identifier")
	}
	if image == "" {
		return nil, errors.New("A container must have an image locator")
	}
	if input == nil {
		input = emptyReader
	}
	if output == nil {
		output = ioutil.Discard
	}
	return &createContainerJobRequest{jobRequest{reqid}, id, image, output}, nil
}
