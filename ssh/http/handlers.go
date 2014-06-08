package http

import (
	"encoding/json"
	"github.com/openshift/go-json-rest"
	"io"

	"github.com/openshift/geard/http"
	"github.com/openshift/geard/http/client"
	"github.com/openshift/geard/ssh/http/remote"
	sshjobs "github.com/openshift/geard/ssh/jobs"
)

type HttpExtension struct{}

func (h *HttpExtension) Routes() http.ExtensionMap {
	return http.ExtensionMap{
		&remote.HttpCreateKeysRequest{}: HandleCreateKeysRequest,
	}
}
func (h *HttpExtension) HttpJobFor(job interface{}) (exc client.RemoteExecutable, err error) {
	return remote.HttpJobFor(job)
}

func HandleCreateKeysRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
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
