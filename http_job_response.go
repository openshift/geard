package geard

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

type HttpJobResponse struct {
	AllowStreaming bool

	succeeded bool
	failed    bool
	response  http.ResponseWriter
	pending   map[string]string
}

func (t JobResponseSuccess) StatusCode() int {
	switch t {
	case JobResponseAccepted:
		return http.StatusAccepted
	default:
		return http.StatusOK
	}
}

func (s *HttpJobResponse) StreamResult() bool {
	return s.AllowStreaming
}

func (s *HttpJobResponse) Success(t JobResponseSuccess) {
	if s.failed {
		panic("Cannot call Success() after failure")
	}
	if s.succeeded {
		panic("Cannot call Success() twice")
	}
	if s.pending != nil {
		header := s.response.Header()
		for key := range s.pending {
			header.Add(key, s.pending[key])
		}
		s.pending = nil
	}
	s.succeeded = true
	s.response.WriteHeader(t.StatusCode())
}
func (s *HttpJobResponse) SuccessWithData(t JobResponseSuccess, data interface{}) {
	s.response.Header().Add("Content-Type", "application/json")
	s.Success(t)
	encoder := json.NewEncoder(s.response)
	encoder.Encode(&data)
}

func (s *HttpJobResponse) SuccessAndWrite(t JobResponseSuccess, flush bool) io.Writer {
	s.Success(t)
	var w io.Writer
	if !s.AllowStreaming {
		w = ioutil.Discard
	} else if flush {
		w = NewWriteFlusher(s.response)
	} else {
		w = s.response
	}
	return w
}

func (s *HttpJobResponse) WriteCloser() <-chan bool {
	if c, ok := s.response.(http.CloseNotifier); ok {
		return c.CloseNotify()
	}
	ch := make(chan bool)
	close(ch)
	return ch
}

func (s *HttpJobResponse) WritePendingSuccess(name string, value interface{}) {
	if s.pending == nil {
		s.pending = make(map[string]string)
	}
	if h, ok := value.(HeaderSerialization); ok {
		s.pending[name] = h.ToHeader()
	} else {
		panic("Passed value does not implement HeaderSerialization for http")
	}
}

func (s *HttpJobResponse) Failure(reason error) {
	if s.succeeded {
		panic("May not invoke failure after Success()")
	}
	if s.failed {
		panic("May not write failure twice")
	}
	s.failed = true
	code := http.StatusInternalServerError
	response := httpFailureResponse{reason.Error(), nil}
	if j, ok := reason.(JobError); ok {
		switch j.Failure() {
		case JobResponseAlreadyExists:
			code = http.StatusConflict
		}
	}
	s.response.WriteHeader(code)
	json.NewEncoder(s.response).Encode(&response)
}

type httpFailureResponse struct {
	Message string      `json:message`
	Data    interface{} `json:data,omitempty`
}

type HeaderSerialization interface {
	ToHeader() string
}

type StringHeader string

func (s StringHeader) ToHeader() string {
	return string(s)
}
