package http

import (
	"encoding/json"
	"github.com/openshift/go-json-rest"
	"io"

	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"
	sshjobs "github.com/openshift/geard/ssh/jobs"
)

type HttpExtension struct{}

func (h *HttpExtension) Routes() []http.HttpJobHandler {
	return []http.HttpJobHandler{
		&HttpCreateKeysRequest{},
	}
}

func (h *HttpExtension) HttpJobFor(job interface{}) (exc http.RemoteExecutable, err error) {
	switch j := job.(type) {
	case *sshjobs.CreateKeysRequest:
		exc = &HttpCreateKeysRequest{CreateKeysRequest: *j}
	default:
		err = jobs.ErrNoJobForRequest
	}
	return
}

type HttpCreateKeysRequest struct {
	sshjobs.CreateKeysRequest
	http.DefaultRequest
}

func (h *HttpCreateKeysRequest) HttpMethod() string { return "PUT" }
func (h *HttpCreateKeysRequest) HttpPath() string   { return "/keys" }
func (h *HttpCreateKeysRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		data := sshjobs.ExtendedCreateKeysData{}
		if r.Body != nil {
			dec := json.NewDecoder(io.LimitReader(r.Body, 100*1024))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		if err := data.Check(); err != nil {
			return nil, err
		}
		return &sshjobs.CreateKeysRequest{
			&data,
		}, nil
	}
}
