package remote

import (
	"github.com/openshift/geard/http/client"
	"github.com/openshift/geard/jobs"
	sshjobs "github.com/openshift/geard/ssh/jobs"
)

func HttpJobFor(job interface{}) (exc client.RemoteExecutable, err error) {
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
	client.DefaultRequest
}

func (h *HttpCreateKeysRequest) HttpMethod() string { return "PUT" }
func (h *HttpCreateKeysRequest) HttpPath() string   { return "/keys" }
