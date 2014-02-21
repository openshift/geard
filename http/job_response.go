package http

import (
	"encoding/json"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/utils"
	"io"
	"io/ioutil"
	"net/http"
)

type httpJobResponse struct {
	response      http.ResponseWriter
	skipStreaming bool
	succeeded     bool
	failed        bool
	pending       map[string]string
}

func NewHttpJobResponse(w http.ResponseWriter, skipStreaming bool) jobs.JobResponse {
	return &httpJobResponse{
		response:      w,
		skipStreaming: skipStreaming,
	}
}

func (s *httpJobResponse) StatusCode(t jobs.JobResponseSuccess) int {
	switch t {
	case jobs.JobResponseAccepted:
		return http.StatusAccepted
	default:
		return http.StatusOK
	}
}

func (s *httpJobResponse) StreamResult() bool {
	return !s.skipStreaming
}

func (s *httpJobResponse) Success(t jobs.JobResponseSuccess) {
	if s.failed {
		panic("Cannot call Success() after failure")
	}
	if s.succeeded {
		panic("Cannot call Success() twice")
	}
	if s.pending != nil {
		header := s.response.Header()
		for key := range s.pending {
			header.Add("x-"+key, s.pending[key])
		}
		s.pending = nil
	}
	s.succeeded = true
	s.response.WriteHeader(s.StatusCode(t))
}

func (s *httpJobResponse) SuccessWithData(t jobs.JobResponseSuccess, data interface{}) {
	s.response.Header().Add("Content-Type", "application/json")
	s.Success(t)
	encoder := json.NewEncoder(s.response)
	encoder.Encode(&data)
}

func (s *httpJobResponse) SuccessWithWrite(t jobs.JobResponseSuccess, flush bool) io.Writer {
	s.Success(t)
	var w io.Writer
	if s.skipStreaming {
		w = ioutil.Discard
	} else if flush {
		w = utils.NewWriteFlusher(s.response)
	} else {
		w = s.response
	}
	return w
}

func (s *httpJobResponse) WriteClosed() <-chan bool {
	if c, ok := s.response.(http.CloseNotifier); ok {
		return c.CloseNotify()
	}
	ch := make(chan bool)
	close(ch)
	return ch
}

func (s *httpJobResponse) WritePendingSuccess(name string, value interface{}) {
	if s.pending == nil {
		s.pending = make(map[string]string)
	}
	if h, ok := value.(HeaderSerialization); ok {
		s.pending[name] = h.ToHeader()
	} else {
		panic("Passed value does not implement HeaderSerialization for http")
	}
}

func (s *httpJobResponse) Failure(e jobs.JobError) {
	if s.succeeded {
		panic("May not invoke failure after Success()")
	}
	if s.failed {
		panic("May not write failure twice")
	}
	s.failed = true

	response := httpFailureResponse{e.Error(), e.ResponseData()}
	var code int
	switch e.ResponseFailure() {
	case jobs.JobResponseAlreadyExists:
		code = http.StatusConflict
	case jobs.JobResponseNotFound:
		code = http.StatusNotFound
	case jobs.JobResponseInvalidRequest:
		code = http.StatusBadRequest
	default:
		code = http.StatusInternalServerError
	}

	s.response.WriteHeader(code)
	json.NewEncoder(s.response).Encode(&response)
}

type httpFailureResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type HeaderSerialization interface {
	ToHeader() string
}

type StringHeader string

func (s StringHeader) ToHeader() string {
	return string(s)
}
