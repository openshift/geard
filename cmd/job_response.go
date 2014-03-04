package cmd

import (
	"encoding/json"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/utils"
	"io"
)

type CliJobResponse struct {
	stdout    io.Writer
	stderr    io.Writer
	succeeded bool
	failed    bool
	exitCode  int
	message   string
}

func (s *CliJobResponse) StreamResult() bool {
	return true
}

func (s *CliJobResponse) Success(t jobs.JobResponseSuccess) {
	if s.failed {
		panic("Cannot call Success() after failure")
	}
	if s.succeeded {
		panic("Cannot call Success() twice")
	}
	s.succeeded = true
	s.exitCode = 0
}

func (s *CliJobResponse) SuccessWithData(t jobs.JobResponseSuccess, data interface{}) {
	s.Success(t)
	encoder := json.NewEncoder(s.stdout)
	encoder.Encode(&data)
}

func (s *CliJobResponse) SuccessWithWrite(t jobs.JobResponseSuccess, flush, structured bool) io.Writer {
	s.Success(t)
	return utils.NewWriteFlusher(s.stdout)
}

func (s *CliJobResponse) WriteClosed() <-chan bool {
	ch := make(chan bool)
	return ch
}

func (s *CliJobResponse) WritePendingSuccess(name string, value interface{}) {
}

func (s *CliJobResponse) Failure(e jobs.JobError) {
	if s.succeeded {
		panic("May not invoke failure after Success()")
	}
	if s.failed {
		panic("May not write failure twice")
	}
	s.failed = true

	var code int
	switch e.ResponseFailure() {
	default:
		code = 1
	}
	s.exitCode = code
	s.message = e.Error()
}
