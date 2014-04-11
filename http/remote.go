package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"

	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/transport"
)

type Locator interface {
	IsRemote() bool
	Identity() string
}

type RemoteLocator interface {
	transport.ResourceLocator
	BaseURL() *url.URL
}

type RemoteJob interface {
	HttpMethod() string
	HttpPath() string
}
type RemoteExecutable interface {
	RemoteJob
	MarshalRequestIdentifier() jobs.RequestIdentifier
	MarshalUrlQuery(*url.Values)
	MarshalHttpRequestBody(io.Writer) error
	UnmarshalHttpResponse(headers http.Header, r io.Reader, mode ResponseContentMode) (interface{}, error)
}

type HttpDispatcher struct {
	client  *http.Client
	locator RemoteLocator
	log     *log.Logger
}

type HttpTransport struct{}

func (h *HttpTransport) NewDispatcher(locator transport.ResourceLocator, logger *log.Logger) (dispatcher transport.Dispatcher, err error) {
	log.Printf("NewDispatcher(%v)", locator)
	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}
	l, ok := locator.(RemoteLocator)
	if !ok {
		err = fmt.Errorf("Invalid locator")
		return
	}

	dispatcher = transport.Dispatcher(&HttpDispatcher{
		client:  &http.Client{},
		locator: l,
		log:     logger,
	})
	return
}

func (h *HttpTransport) RequestFor(job jobs.Job) transport.TransportRequest {
	switch j := job.(type) {
	case *jobs.InstallContainerRequest:
		return &HttpInstallContainerRequest{InstallContainerRequest: *j}
	case *jobs.StartedContainerStateRequest:
		return &HttpStartContainerRequest{StartedContainerStateRequest: *j}
	case *jobs.StoppedContainerStateRequest:
		return &HttpStopContainerRequest{StoppedContainerStateRequest: *j}
	case *jobs.RestartContainerRequest:
		return &HttpRestartContainerRequest{RestartContainerRequest: *j}
	case *jobs.PutEnvironmentRequest:
		return &HttpPutEnvironmentRequest{PutEnvironmentRequest: *j}
	case *jobs.PatchEnvironmentRequest:
		return &HttpPatchEnvironmentRequest{PatchEnvironmentRequest: *j}
	case *jobs.ContainerStatusRequest:
		return &HttpContainerStatusRequest{ContainerStatusRequest: *j}
	case *jobs.ContentRequest:
		return &HttpContentRequest{ContentRequest: *j}
	case *jobs.DeleteContainerRequest:
		return &HttpDeleteContainerRequest{DeleteContainerRequest: *j}
	case *jobs.LinkContainersRequest:
		return &HttpLinkContainersRequest{LinkContainersRequest: *j}
	case *jobs.ListContainersRequest:
		return &HttpListContainersRequest{ListContainersRequest: *j}
	default:
		for _, ext := range extensions {
			if req := ext.RequestFor(job); req != nil {
				return req
			}
		}
		log.Printf("Unable to process job type %v", reflect.TypeOf(j))
	}
	return nil
}

func (h *HttpDispatcher) Dispatch(j transport.RemoteExecutable, res jobs.JobResponse) error {
	job, ok := j.(RemoteExecutable)
	if !ok {
		return fmt.Errorf("Invalid job")
	}

	reader, writer := io.Pipe()
	httpreq, errn := http.NewRequest(job.HttpMethod(), h.locator.BaseURL().String(), reader)
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
	req.Header.Set("If-Match", "api="+ApiVersion())
	req.Header.Set("Content-Type", "application/json")
	//TODO: introduce API version per job
	//TODO: content request signing for GETs
	req.URL.Path = job.HttpPath()
	req.URL.RawQuery = query.Encode()
	go func() {
		if err := job.MarshalHttpRequestBody(writer); err != nil {
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
		data, err := job.UnmarshalHttpResponse(resp.Header, nil, ResponseTable)
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
		data, err := job.UnmarshalHttpResponse(resp.Header, nil, ResponseTable)
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
		data, err := job.UnmarshalHttpResponse(nil, resp.Body, ResponseJson)
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
