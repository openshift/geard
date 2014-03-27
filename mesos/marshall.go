// +build mesos

package mesos

import (
	"github.com/openshift/geard/jobs"

	"errors"
	"io"
)

type DefaultRequest struct {
}

func (r *DefaultRequest) MarshalRequestBody(w io.Writer) error {
	return nil
}

func (h *DefaultRequest) MarshalRequestIdentifier() jobs.RequestIdentifier {
	return jobs.RequestIdentifier{}
}

func (h *DefaultRequest) UnmarshalResponse(r io.Reader) (interface{}, error) {
	if r != nil {
		return nil, errors.New("Unexpected response body to request")
	}
	return nil, nil
}
