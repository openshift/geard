package http

import (
	"encoding/json"
	"errors"
	"io"
)

func (h *DefaultRequest) MarshalToHttp(w io.Writer) error {
	return nil
}
func (h *DefaultRequest) MarshalToToken(token *TokenData) {
}
func (h *DefaultRequest) MarshalResponse(r io.Reader, mode ResponseContentMode) (interface{}, error) {
	return nil, errors.New("Unexpected response body to request")
}

func (h *HttpInstallContainerRequest) MarshalToHttp(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}
func (h *HttpInstallContainerRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
	token.T = h.Image
}
func (h *HttpInstallContainerRequest) MarshalResponse(r io.Reader, mode ResponseContentMode) (interface{}, error) {
	return nil, errors.New("Unexpected response body to HttpInstallContainerRequest")
}

func (h *HttpStartContainerRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
}

func (h *HttpStopContainerRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
}
