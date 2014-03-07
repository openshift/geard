package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/jobs"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
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
	MarshalHttpResponse(headers http.Header, r io.Reader, mode ResponseContentMode) (interface{}, error)
}

type HttpDispatcher struct {
	client  *http.Client
	locator RemoteLocator
	log     *log.Logger
}

func NewHttpDispatcher(l RemoteLocator, logger *log.Logger) *HttpDispatcher {
	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}
	return &HttpDispatcher{
		client:  &http.Client{},
		locator: l,
		log:     logger,
	}
}

func (h *HttpDispatcher) Dispatch(job RemoteExecutable, res jobs.JobResponse) error {
	reader, writer := io.Pipe()
	httpreq, errn := http.NewRequest(job.HttpMethod(), h.locator.BaseURL().String(), reader)
	if errn != nil {
		return errn
	}
	token := &TokenData{}
	job.MarshalToToken(token)
	if len(token.I) == 0 {
		token.SetRequestIdentifier(jobs.NewRequestIdentifier())
	}
	token.D = int(time.Now().Unix())
	query := &url.Values{}
	token.ToValues(query)

	req := httpreq
	req.Header.Add("If-Match", "api="+ApiVersion())
	req.Header.Add("Content-Type", "application/json")
	req.URL.Path = "/token/__test__" + job.HttpPath()
	req.URL.RawQuery = query.Encode()
	go func() {
		if err := job.MarshalToHttp(writer); err != nil {
			h.log.Printf("remote: Error when writing to http: %v", err)
			writer.CloseWithError(err)
		} else {
			writer.Close()
		}
	}()

	resp, err := h.client.Do(req)
	if err != nil {
		h.log.Printf("Failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	isJson := resp.Header.Get("Content-Type") == "application/json"

	switch code := resp.StatusCode; {
	case code == 202:
		if isJson {
			return errors.New("Decoding of streaming JSON has not been implemented")
		}
		data, err := job.MarshalHttpResponse(resp.Header, nil, ResponseTable)
		if err != nil {
			return err
		}
		if pending, ok := data.(map[string]interface{}); ok {
			for k := range pending {
				res.WritePendingSuccess(k, pending[k])
			}
		}
		w := res.SuccessWithWrite(jobs.JobResponseOk, false, false)
		if _, err := io.Copy(w, resp.Body); err != nil {
			return err
		}
	case code == 204:
		data, err := job.MarshalHttpResponse(resp.Header, nil, ResponseTable)
		if err != nil {
			return err
		}
		if pending, ok := data.(map[string]interface{}); ok {
			for k := range pending {
				res.WritePendingSuccess(k, pending[k])
			}
		}
		res.Success(jobs.JobResponseOk)
	case code >= 200 && code < 300:
		if !isJson {
			return errors.New(fmt.Sprintf("remote: Response with %d status code had content type %s (should be application/json)", code, resp.Header.Get("Content-Type")))
		}
		data, err := job.MarshalHttpResponse(nil, resp.Body, ResponseJson)
		if err != nil {
			return err
		}
		res.SuccessWithData(jobs.JobResponseOk, data)
	default:
		if isJson {
			decoder := json.NewDecoder(resp.Body)
			data := httpFailureResponse{}
			if err := decoder.Decode(&data); err != nil {
				return err
			}
			res.Failure(jobs.SimpleJobError{jobs.JobResponseError, data.Message})
			return nil
		}
		io.Copy(os.Stderr, resp.Body)
		res.Failure(jobs.SimpleJobError{jobs.JobResponseError, "Unable to decode response."})
	}
	return nil
}
