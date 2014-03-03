package http

import (
	"encoding/json"
	"io"
)

func (h *HttpInstallContainerRequest) MarshalToHttp(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}
func (h *HttpInstallContainerRequest) MarshalToToken(token *TokenData) {
	token.R = string(h.Id)
	token.T = h.Image
}
