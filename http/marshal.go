package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/port"
	"io"
	"net/http"
	"net/url"
	"text/tabwriter"
)

func (h *DefaultRequest) MarshalHttpRequestBody(w io.Writer) error {
	return nil
}
func (h *DefaultRequest) MarshalRequestIdentifier() jobs.RequestIdentifier {
	return jobs.RequestIdentifier{}
}
func (h *DefaultRequest) MarshalUrlQuery(query *url.Values) {
}
func (h *DefaultRequest) UnmarshalHttpResponse(headers http.Header, r io.Reader, mode ResponseContentMode) (interface{}, error) {
	if r != nil {
		return nil, errors.New("Unexpected response body to request")
	}
	return nil, nil
}

func (h *HttpRunContainerRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h.RunContainerRequest)
}

func (h *HttpInstallContainerRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}
func (h *HttpInstallContainerRequest) UnmarshalHttpResponse(headers http.Header, r io.Reader, mode ResponseContentMode) (interface{}, error) {
	if r == nil {
		pending := make(map[string]interface{})
		if s := headers.Get("X-" + jobs.PendingPortMappingName); s != "" {
			ports, err := port.FromPortPairHeader(s)
			if err != nil {
				return nil, err
			}
			pending[jobs.PendingPortMappingName] = ports
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
type ListContainersResponse struct {
	jobs.ListContainersResponse
}

func (l *ListContainersResponse) WriteTableTo(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 8, 4, 1, ' ', tabwriter.DiscardEmptyColumns)
	if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", "ID", "SERVER", "ACTIVE", "SUB", "LOAD", "TYPE"); err != nil {
		return err
	}
	for i := range l.Containers {
		container := &l.Containers[i]
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", container.Id, container.Server, container.ActiveState, container.SubState, container.LoadState, container.JobType); err != nil {
			return err
		}
	}
	tw.Flush()
	return nil
}

func (h *HttpListContainersRequest) UnmarshalHttpResponse(headers http.Header, r io.Reader, mode ResponseContentMode) (interface{}, error) {
	if r == nil {
		return nil, errors.New("Unexpected empty response body to HttpListContainersRequest")
	}
	decoder := json.NewDecoder(r)
	list := &ListContainersResponse{}
	if err := decoder.Decode(list); err != nil {
		return nil, err
	}
	for i := range list.Containers {
		c := &list.Containers[i]
		c.Server = h.Label
	}
	return list, nil
}
