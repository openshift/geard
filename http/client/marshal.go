package client

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/utils"
)

type DefaultRequest struct {
	Server string `json:"-"`
}

func (h *DefaultRequest) SetServer(server string) {
	h.Server = server
}
func (h *DefaultRequest) Streamable() bool {
	return true
}
func (h *DefaultRequest) HttpApiVersion() string {
	return "1"
}
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

var reSplat = regexp.MustCompile("\\:[a-z\\*]+")

func Inline(s string, with ...string) string {
	match := 0
	return string(reSplat.ReplaceAllFunc([]byte(s), func(p []byte) []byte {
		repl := with[match]
		match += 1
		if repl == "" {
			return p
		} else {
			return []byte(utils.EncodeUrlPath(repl))
		}
	}))
}
