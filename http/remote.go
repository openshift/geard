package http

import (
	"github.com/smarterclayton/geard/jobs"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type Locator interface {
	IsRemote() bool
	Identity() string
}

type RemoteLocator interface {
	BaseURL() *url.URL
}

type RemoteJob interface {
	HttpMethod() string
	HttpPath() string
}
type RemoteExecutable interface {
	RemoteJob
	MarshalToToken(token *TokenData)
	MarshalToHttp(io.Writer) error
}

type HttpDispatcher struct {
	client  *http.Client
	locator RemoteLocator
}

func NewHttpDispatcher(l RemoteLocator) *HttpDispatcher {
	return &HttpDispatcher{
		&http.Client{},
		l,
	}
}

func (h *HttpDispatcher) Dispatch(job RemoteExecutable) error {
	reader, writer := io.Pipe()
	httpreq, errn := http.NewRequest(job.HttpMethod(), h.locator.BaseURL().String(), reader)
	if errn != nil {
		return errn
	}
	token := &TokenData{}
	job.MarshalToToken(token)
	token.SetRequestIdentifier(jobs.NewRequestIdentifier())
	token.D = int(time.Now().Unix())
	query := &url.Values{}
	token.ToValues(query)

	req := RemoteHttpRequest{httpreq}
	req.request.Header.Add("If-Match", "api="+ApiVersion())
	req.request.Header.Add("Content-Type", "application/json")
	req.request.URL.Path = "/token/__test__" + job.HttpPath()
	req.request.URL.RawQuery = query.Encode()
	go func() {
		if err := job.MarshalToHttp(writer); err != nil {
			log.Printf("remote: Error when writing to http: %v", err)
			writer.CloseWithError(err)
		} else {
			writer.Close()
		}
	}()
	log.Printf("Sending request to %v", req.request)
	resp, err := h.client.Do(req.request)
	if err != nil {
		log.Printf("Failed: %v", err)
		return err
	}
	log.Printf("Closing %v", resp)
	defer resp.Body.Close()
	return nil
}

type RemoteHttpRequest struct {
	request *http.Request
}
