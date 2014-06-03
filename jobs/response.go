package jobs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sort"

	"github.com/openshift/geard/utils"
)

// A default implementation of the Response interface.
type ClientResponse struct {
	// A response stream to output to.  Defaults to DevNull
	Output io.Writer
	// true if output should be captured rather than printed
	Gather bool

	// Data gathered during request parsing FIXME: move to marshal?
	Pending map[string]interface{}
	// Data gathered from the response
	Data interface{}
	// The error set on the response
	Error error

	succeeded bool
	failed    bool
}

type printable interface {
	String() string
}

func (s *ClientResponse) StreamResult() bool {
	return true
}

func (s *ClientResponse) Success(t ResponseSuccess) {
	if s.failed {
		s.Error = errors.New("jobs: Cannot call Success() after failure")
		return
	}
	if s.succeeded {
		s.Error = errors.New("jobs: Cannot call Success() twice")
		return
	}
	s.succeeded = true
	if !s.Gather {
		s.WritePending(s.Output)
	}
}

func (s *ClientResponse) SuccessWithData(t ResponseSuccess, data interface{}) {
	s.Success(t)
	s.Data = data
	if !s.Gather {
		encoder := json.NewEncoder(s.Output)
		encoder.Encode(&data)
	}
}

func (s *ClientResponse) SuccessWithWrite(t ResponseSuccess, flush, structured bool) io.Writer {
	s.Success(t)
	if s.Gather {
		if structured {
			s.Error = errors.New("jobs: Client does not support receiving streaming structured data")
			return ioutil.Discard
		}
		buf := bytes.Buffer{}
		s.Data = &buf
		return &buf
	}
	return utils.NewWriteFlusher(s.Output)
}

func (s *ClientResponse) WritePendingSuccess(name string, value interface{}) {
	if s.Pending == nil {
		s.Pending = make(map[string]interface{})
	}
	s.Pending[name] = value
}

func (s *ClientResponse) Failure(e error) {
	if s.succeeded {
		s.Error = errors.New("jobs: may not invoke failure after Success()")
		return
	}
	if s.failed {
		s.Error = errors.New("jobs: may not write failure twice")
		return
	}
	s.failed = true
	s.Error = e
}

func (s *ClientResponse) WritePending(w io.Writer) {
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
