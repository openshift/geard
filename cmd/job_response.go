package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/utils"
	"io"
	"sort"
)

type CliJobResponse struct {
	stdout    io.Writer
	stderr    io.Writer
	succeeded bool
	failed    bool
	exitCode  int
	message   string
	pending   map[string]interface{}
}

type printable interface {
	String() string
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
	if s.pending == nil {
		s.pending = make(map[string]interface{})
	}
	s.pending[name] = value
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

func (s *CliJobResponse) WritePending(w io.Writer) {
	if s.pending != nil {
		keys := make([]string, 0, len(s.pending))
		for k := range s.pending {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i := range keys {
			k := keys[i]
			v := s.pending[k]
			if prints, ok := v.(printable); ok {
				fmt.Fprintf(w, "%s: %s\n", k, prints.String())
			} else {
				fmt.Fprintf(w, "%s: %+v\n", k, v)
			}
		}
	}
}
