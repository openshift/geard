package remote

import (
	"encoding/json"
	"io"
)

func (h *HttpCreateKeysRequest) MarshalHttpRequestBody(w io.Writer) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(h)
}
