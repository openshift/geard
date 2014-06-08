package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/openshift/geard/jobs"
)

const DefaultHttpPort = "43273"

type ResponseContentMode int

const (
	ResponseJson ResponseContentMode = iota
	ResponseTable
)

type HttpFailureResponse struct {
	Message string
	Data    interface{} `json:"Data,omitempty"`
	// FIXME: default error reporter can include this
	Error string `json:"Error,omitempty"`
}

type RemoteJob interface {
	HttpMethod() string
	HttpPath() string
	HttpApiVersion() string
}
type RemoteExecutable interface {
	RemoteJob
	MarshalRequestIdentifier() jobs.RequestIdentifier
	MarshalUrlQuery(*url.Values)
	MarshalHttpRequestBody(io.Writer) error
	UnmarshalHttpResponse(headers http.Header, r io.Reader, mode ResponseContentMode) (interface{}, error)
}
type HttpStreamable interface {
	Streamable() bool
}

type HttpClient struct {
	Client http.Client
}

func (h *HttpClient) ExecuteRemote(baseUrl *url.URL, job RemoteExecutable, res jobs.Response) error {
	reader, writer := io.Pipe()
	httpreq, errn := http.NewRequest(job.HttpMethod(), baseUrl.String(), reader)
	if errn != nil {
		return errn
	}

	id := job.MarshalRequestIdentifier()
	if len(id) == 0 {
		id = jobs.NewRequestIdentifier()
	}

	query := &url.Values{}
	job.MarshalUrlQuery(query)

	req := httpreq
	req.Header.Set("X-Request-Id", id.String())
	req.Header.Set("X-Api-Version", job.HttpApiVersion())

	if streamable, ok := job.(HttpStreamable); ok && streamable.Streamable() {
		req.Header.Set("Accept", "application/json;stream=true")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	//TODO: introduce API version per job
	//TODO: content request signing for GETs
	req.URL.Path = job.HttpPath()
	req.URL.RawQuery = query.Encode()
	go func() {
		if err := job.MarshalHttpRequestBody(writer); err != nil {
			log.Printf("http_remote: Error when writing to http: %v", err)
			writer.CloseWithError(err)
		} else {
			writer.Close()
		}
	}()

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	isJson := resp.Header.Get("Content-Type") == "application/json"

	switch code := resp.StatusCode; {
	case code == 202:
		if isJson {
			return errors.New("Decoding of streaming JSON has not been implemented")
		}
		data, err := job.UnmarshalHttpResponse(resp.Header, nil, ResponseTable)
		if err != nil {
			return err
		}
		if pending, ok := data.(map[string]interface{}); ok {
			for k := range pending {
				res.WritePendingSuccess(k, pending[k])
			}
		}
		w := res.SuccessWithWrite(jobs.ResponseOk, false, false)
		if _, err := io.Copy(w, resp.Body); err != nil {
			return err
		}
	case code == 204:
		data, err := job.UnmarshalHttpResponse(resp.Header, nil, ResponseTable)
		if err != nil {
			return err
		}
		if pending, ok := data.(map[string]interface{}); ok {
			for k := range pending {
				res.WritePendingSuccess(k, pending[k])
			}
		}
		res.Success(jobs.ResponseOk)
	case code >= 200 && code < 300:
		if !isJson {
			return errors.New(fmt.Sprintf("remote: Response with %d status code had content type %s (should be application/json)", code, resp.Header.Get("Content-Type")))
		}
		data, err := job.UnmarshalHttpResponse(nil, resp.Body, ResponseJson)
		if err != nil {
			return err
		}
		res.SuccessWithData(jobs.ResponseOk, data)
	default:
		if isJson {
			decoder := json.NewDecoder(resp.Body)
			data := HttpFailureResponse{}
			if err := decoder.Decode(&data); err != nil {
				return err
			}
			if data.Message == "" && data.Error != "" {
				data.Message = fmt.Sprintf("The server did not respond correctly to the request: %s", data.Error)
			}
			res.Failure(jobs.SimpleError{jobs.ResponseError, data.Message})
			return nil
		}
		io.Copy(os.Stderr, resp.Body)
		res.Failure(jobs.SimpleError{jobs.ResponseError, "Unable to decode response."})
	}
	return nil
}
