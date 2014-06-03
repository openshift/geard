package remote

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/http/client"
	"github.com/openshift/geard/port"
)

func (h *HttpRunContainerRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h.RunContainerRequest)
}

func (h *HttpInstallContainerRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}
func (h *HttpInstallContainerRequest) UnmarshalHttpResponse(headers http.Header, r io.Reader, mode client.ResponseContentMode) (interface{}, error) {
	if r == nil {
		pending := make(map[string]interface{})
		if s := headers.Get("X-" + cjobs.PendingPortMappingName); s != "" {
			ports, err := port.FromPortPairHeader(s)
			if err != nil {
				return nil, err
			}
			pending[cjobs.PendingPortMappingName] = ports
		}
		return pending, nil
	}
	return nil, errors.New("Unexpected response body to HttpInstallContainerRequest")
}

func (h *HttpPutEnvironmentRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h.EnvironmentDescription)
}

func (h *HttpPatchEnvironmentRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h.EnvironmentDescription)
}

func (h *HttpLinkContainersRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h.LinkContainersRequest)
}

// Apply the "label" from the job to the response
func (h *HttpListContainersRequest) UnmarshalHttpResponse(headers http.Header, r io.Reader, mode client.ResponseContentMode) (interface{}, error) {
	if r == nil {
		return nil, errors.New("Unexpected empty response body to HttpListContainersRequest")
	}
	decoder := json.NewDecoder(r)
	list := &cjobs.ListContainersResponse{}
	if err := decoder.Decode(list); err != nil {
		return nil, err
	}
	for i := range list.Containers {
		c := &list.Containers[i]
		c.Server = h.Server
	}
	return list, nil
}
