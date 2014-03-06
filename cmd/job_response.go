package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/utils"
	"io"
	"sort"
)

type CliJobResponse struct {
	// A response stream to output to.  Defaults to DevNull
	Output io.Writer
	// true if output should be captured rather than printed
	Gather bool

	// Data gathered during request parsing FIXME: move to marshal?
	Pending map[string]interface{}
	// Data gathered from the response
	Data interface{}
	// The error set on the response
	Error jobs.JobError

	succeeded bool
	failed    bool
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
	if !s.Gather {
		s.WritePending(s.Output)
	}
}

func (s *CliJobResponse) SuccessWithData(t jobs.JobResponseSuccess, data interface{}) {
	s.Success(t)
	s.Data = data
	if !s.Gather {
		encoder := json.NewEncoder(s.Output)
		encoder.Encode(&data)
	}
}

func (s *CliJobResponse) SuccessWithWrite(t jobs.JobResponseSuccess, flush, structured bool) io.Writer {
	s.Success(t)
	if s.Gather {
		if structured {
			panic("Client does not support receiving streaming structured data.")
		}
		buf := bytes.Buffer{}
		s.Data = &buf
		return &buf
	}
	return utils.NewWriteFlusher(s.Output)
}

func (s *CliJobResponse) WriteClosed() <-chan bool {
	ch := make(chan bool)
	return ch
}

func (s *CliJobResponse) WritePendingSuccess(name string, value interface{}) {
	if s.Pending == nil {
		s.Pending = make(map[string]interface{})
	}
	s.Pending[name] = value
}

func (s *CliJobResponse) Failure(e jobs.JobError) {
	if s.succeeded {
		panic("May not invoke failure after Success()")
	}
	if s.failed {
		panic("May not write failure twice")
	}
	s.failed = true
	s.Error = e
}

func (s *CliJobResponse) WritePending(w io.Writer) {
	if s.Pending != nil {
		keys := make([]string, 0, len(s.Pending))
		for k := range s.Pending {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i := range keys {
			k := keys[i]
			v := s.Pending[k]
			if prints, ok := v.(printable); ok {
				fmt.Fprintf(w, "%s: %s\n", k, prints.String())
			} else {
				fmt.Fprintf(w, "%s: %+v\n", k, v)
			}
		}
	}
}
