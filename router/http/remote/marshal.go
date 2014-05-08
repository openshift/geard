package remote

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"github.com/openshift/geard/http/client"
)

func (h *HttpRouterCreateFrontendRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}

func (h *HttpRouterCreateRouteRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}

func (h *HttpRouterGetRoutesRequest) UnmarshalHttpResponse(headers http.Header, r io.Reader, mode client.ResponseContentMode) (interface{}, error) {
	if r == nil {
		return nil, errors.New("Unexpected empty response body to HttpRouterGetRoutesRequest")
	}
	decoder := json.NewDecoder(r)
	var frontendinfo string //router.Frontend
	if err := decoder.Decode(&frontendinfo); err != nil {
		return nil, err
	}
	return frontendinfo, nil
}

func (h *HttpRouterAddAliasRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}
