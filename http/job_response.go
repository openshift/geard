package http

import (
	"encoding/json"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/utils"
	"io"
	"io/ioutil"
	"net/http"
)

var (
	ErrContentTypeDoesNotMatch = jobs.SimpleError{jobs.ResponseNotAcceptable, "The content type you requested is not available for this action."}
)

type ResponseContentMode int

const (
	ResponseJson ResponseContentMode = iota
	ResponseTable
)

type TabularOutput interface {
	WriteTableTo(io.Writer) error
}

type httpJobResponse struct {
	response      http.ResponseWriter
	skipStreaming bool
	mode          ResponseContentMode
	succeeded     bool
	failed        bool
	pending       map[string]string
}

func NewHttpJobResponse(w http.ResponseWriter, skipStreaming bool, mode ResponseContentMode) jobs.Response {
	return &httpJobResponse{
		response:      w,
		skipStreaming: skipStreaming,
		mode:          mode,
	}
}

func (s *httpJobResponse) StreamResult() bool {
	return !s.skipStreaming
}

func (s *httpJobResponse) Success(t jobs.ResponseSuccess) {
	s.success(t, false, false)
}

func (s *httpJobResponse) SuccessWithData(t jobs.ResponseSuccess, data interface{}) {
	if s.mode == ResponseTable {
		tabular, ok := data.(TabularOutput)
		if !ok {
			s.Failure(ErrContentTypeDoesNotMatch)
			return
		}
		s.response.Header().Add("Content-Type", "text/plain")
		s.success(t, false, true)
		tabular.WriteTableTo(s.response)
		return
	}
	s.response.Header().Add("Content-Type", "application/json")
	s.success(t, false, true)
	encoder := json.NewEncoder(s.response)
	encoder.Encode(&data)
}

func (s *httpJobResponse) SuccessWithWrite(t jobs.ResponseSuccess, flush, structured bool) io.Writer {
	if structured {
		s.response.Header().Add("Content-Type", "application/json")
	} else {
		s.response.Header().Add("Content-Type", "text/plain")
	}
	s.success(t, !s.skipStreaming, false)
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

func (s *httpJobResponse) success(t jobs.ResponseSuccess, stream, data bool) {
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
	s.response.WriteHeader(s.statusCode(t, stream, data))
}

func (s *httpJobResponse) statusCode(t jobs.ResponseSuccess, stream, data bool) int {
	switch {
	case stream:
		return http.StatusAccepted
	case data:
		return http.StatusOK
	default:
		return http.StatusNoContent
	}
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

func (s *httpJobResponse) Failure(err error) {
	if s.succeeded {
		panic("May not invoke failure after Success()")
	}
	if s.failed {
		panic("May not write failure twice")
	}
	s.failed = true

	code := http.StatusInternalServerError
	response := httpFailureResponse{err.Error(), nil}
	s.response.Header().Set("Content-Type", "application/json")

	if e, ok := err.(jobs.JobError); ok {
		response.Data = e.ResponseData()

		switch e.ResponseFailure() {
		case jobs.ResponseAlreadyExists:
			code = http.StatusConflict
		case jobs.ResponseNotFound:
			code = http.StatusNotFound
		case jobs.ResponseInvalidRequest:
			code = http.StatusBadRequest
		case jobs.ResponseNotAcceptable:
			code = http.StatusNotAcceptable
		case jobs.ResponseRateLimit:
			code = 429 // http.statusTooManyRequests
		}
	}

	s.response.WriteHeader(code)
	json.NewEncoder(s.response).Encode(&response)
}

type httpFailureResponse struct {
	Message string
	Data    interface{} `json:"Data,omitempty"`
}

type HeaderSerialization interface {
	ToHeader() string
}

type StringHeader string

func (s StringHeader) ToHeader() string {
	return string(s)
}
