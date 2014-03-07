package http

import (
	"encoding/json"
	"errors"
	"github.com/smarterclayton/geard/containers"
	"io"
	"net/http"
)

func (h *DefaultRequest) MarshalToHttp(w io.Writer) error {
	return nil
}
func (h *DefaultRequest) MarshalToToken(token *TokenData) {
}
func (h *DefaultRequest) MarshalHttpResponse(headers http.Header, r io.Reader, mode ResponseContentMode) (interface{}, error) {
	if r != nil {
		return nil, errors.New("Unexpected response body to request")
	}
	return nil, nil
}

func (h *HttpInstallContainerRequest) MarshalToHttp(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}
func (h *HttpInstallContainerRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
	token.T = h.Image
}
func (h *HttpInstallContainerRequest) MarshalHttpResponse(headers http.Header, r io.Reader, mode ResponseContentMode) (interface{}, error) {
	if r == nil {
		pending := make(map[string]interface{})
		if s := headers.Get("X-PortMapping"); s != "" {
			ports, err := containers.FromPortPairHeader(s)
			if err != nil {
				return nil, err
			}
			pending["Ports"] = ports
		}
		return pending, nil
	}
	return nil, errors.New("Unexpected response body to HttpInstallContainerRequest")
}

func (h *HttpDeleteContainerRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
}

func (h *HttpStartContainerRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
}

func (h *HttpStopContainerRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
}

func (h *HttpContainerStatusRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
}

func (h *HttpPutEnvironmentRequest) MarshalToHttp(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h.EnvironmentDescription)
}
func (h *HttpPutEnvironmentRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.EnvironmentDescription.Id)
}

func (h *HttpPatchEnvironmentRequest) MarshalToHttp(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h.EnvironmentDescription)
}
func (h *HttpPatchEnvironmentRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.EnvironmentDescription.Id)
}

func (h *HttpContentRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.ContentRequest.Locator)
	//token.T = string(h.ContentRequest.Type) GOTCHA: Ensure subpath is set for each content request
}
