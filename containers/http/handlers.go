package http

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/openshift/geard/containers"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/go-json-rest"
)

type HttpExtension struct{}

func (h *HttpExtension) Routes() []http.HttpJobHandler {
	return []http.HttpJobHandler{
		&HttpRunContainerRequest{},

		&HttpInstallContainerRequest{},
		&HttpDeleteContainerRequest{},
		&HttpContainerLogRequest{},
		&HttpContainerStatusRequest{},
		&HttpListContainerPortsRequest{},

		&HttpStartContainerRequest{},
		&HttpStopContainerRequest{},
		&HttpRestartContainerRequest{},

		&HttpLinkContainersRequest{},

		&HttpListContainersRequest{},
		&HttpListImagesRequest{},
		&HttpListBuildsRequest{},

		&HttpBuildImageRequest{},

		&HttpPatchEnvironmentRequest{},
		&HttpPutEnvironmentRequest{},

		&HttpContentRequest{},
		&HttpContentRequest{ContentRequest: cjobs.ContentRequest{Subpath: "*"}},
		&HttpContentRequest{ContentRequest: cjobs.ContentRequest{Type: cjobs.ContentTypeEnvironment}},
	}
}

func (h *HttpExtension) HttpJobFor(job interface{}) (exc http.RemoteExecutable, err error) {
	switch j := job.(type) {
	case *cjobs.InstallContainerRequest:
		exc = &HttpInstallContainerRequest{InstallContainerRequest: *j}
	case *cjobs.StartedContainerStateRequest:
		exc = &HttpStartContainerRequest{StartedContainerStateRequest: *j}
	case *cjobs.StoppedContainerStateRequest:
		exc = &HttpStopContainerRequest{StoppedContainerStateRequest: *j}
	case *cjobs.RestartContainerRequest:
		exc = &HttpRestartContainerRequest{RestartContainerRequest: *j}
	case *cjobs.PutEnvironmentRequest:
		exc = &HttpPutEnvironmentRequest{PutEnvironmentRequest: *j}
	case *cjobs.PatchEnvironmentRequest:
		exc = &HttpPatchEnvironmentRequest{PatchEnvironmentRequest: *j}
	case *cjobs.ContainerStatusRequest:
		exc = &HttpContainerStatusRequest{ContainerStatusRequest: *j}
	case *cjobs.ContentRequest:
		exc = &HttpContentRequest{ContentRequest: *j}
	case *cjobs.DeleteContainerRequest:
		exc = &HttpDeleteContainerRequest{DeleteContainerRequest: *j}
	case *cjobs.LinkContainersRequest:
		exc = &HttpLinkContainersRequest{LinkContainersRequest: *j}
	case *cjobs.ListContainersRequest:
		exc = &HttpListContainersRequest{ListContainersRequest: *j}
	default:
		err = jobs.ErrNoJobForRequest
	}
	return
}

func limitedBodyReader(r *rest.Request) io.Reader {
	return io.LimitReader(r.Body, 100*1024)
}

type HttpRunContainerRequest struct {
	cjobs.RunContainerRequest
	http.DefaultRequest
}

func (h *HttpRunContainerRequest) HttpMethod() string { return "POST" }
func (h *HttpRunContainerRequest) HttpPath() string   { return "/jobs" }
func (h *HttpRunContainerRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		data := cjobs.RunContainerRequest{}
		if r.Body != nil {
			dec := json.NewDecoder(limitedBodyReader(r))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		data.Name = context.Id.String()
		if err := data.Check(); err != nil {
			return nil, err
		}
		return &data, nil
	}
}

type HttpInstallContainerRequest struct {
	cjobs.InstallContainerRequest
	http.DefaultRequest
}

func (h *HttpInstallContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpInstallContainerRequest) HttpPath() string {
	return http.Inline("/container/:id", string(h.Id))
}
func (h *HttpInstallContainerRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		data := cjobs.InstallContainerRequest{}
		if r.Body != nil {
			dec := json.NewDecoder(limitedBodyReader(r))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		data.Id = id
		data.RequestIdentifier = context.Id

		if err := data.Check(); err != nil {
			return nil, err
		}
		return &data, nil
	}
}

type HttpDeleteContainerRequest struct {
	cjobs.DeleteContainerRequest
	http.DefaultRequest
}

func (h *HttpDeleteContainerRequest) HttpMethod() string { return "DELETE" }
func (h *HttpDeleteContainerRequest) HttpPath() string {
	return http.Inline("/container/:id", string(h.Id))
}
func (h *HttpDeleteContainerRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.DeleteContainerRequest{Id: id}, nil
	}
}

type HttpListContainersRequest struct {
	cjobs.ListContainersRequest
	http.DefaultRequest
}

func (h *HttpListContainersRequest) HttpMethod() string { return "GET" }
func (h *HttpListContainersRequest) HttpPath() string   { return "/containers" }
func (h *HttpListContainersRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		return &cjobs.ListContainersRequest{}, nil
	}
}

type HttpListBuildsRequest cjobs.ListBuildsRequest

func (h *HttpListBuildsRequest) HttpMethod() string { return "GET" }
func (h *HttpListBuildsRequest) HttpPath() string   { return "/builds" }
func (h *HttpListBuildsRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		return &cjobs.ListBuildsRequest{}, nil
	}
}

type HttpListImagesRequest cjobs.ListImagesRequest

func (h *HttpListImagesRequest) HttpMethod() string { return "GET" }
func (h *HttpListImagesRequest) HttpPath() string   { return "/images" }
func (h *HttpListImagesRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		return &cjobs.ListImagesRequest{conf.Docker.Socket}, nil
	}
}

type HttpContainerLogRequest cjobs.ContainerLogRequest

func (h *HttpContainerLogRequest) HttpMethod() string { return "GET" }
func (h *HttpContainerLogRequest) HttpPath() string {
	return http.Inline("/container/:id/log", string(h.Id))
}
func (h *HttpContainerLogRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.ContainerLogRequest{
			id,
		}, nil
	}
}

type HttpContainerStatusRequest struct {
	cjobs.ContainerStatusRequest
	http.DefaultRequest
}

func (h *HttpContainerStatusRequest) HttpMethod() string { return "GET" }
func (h *HttpContainerStatusRequest) Streamable() bool   { return true }
func (h *HttpContainerStatusRequest) HttpPath() string {
	return http.Inline("/container/:id/status", string(h.Id))
}
func (h *HttpContainerStatusRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.ContainerStatusRequest{Id: id}, nil
	}
}

type HttpListContainerPortsRequest cjobs.ContainerPortsRequest

func (h *HttpListContainerPortsRequest) HttpMethod() string { return "GET" }
func (h *HttpListContainerPortsRequest) HttpPath() string {
	return http.Inline("/container/:id/ports", string(h.Id))
}
func (h *HttpListContainerPortsRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.ContainerPortsRequest{
			id,
		}, nil
	}
}

type HttpStartContainerRequest struct {
	cjobs.StartedContainerStateRequest
	http.DefaultRequest
}

func (h *HttpStartContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpStartContainerRequest) Streamable() bool   { return true }
func (h *HttpStartContainerRequest) HttpPath() string {
	return http.Inline("/container/:id/started", string(h.Id))
}
func (h *HttpStartContainerRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.StartedContainerStateRequest{id}, nil
	}
}

type HttpStopContainerRequest struct {
	cjobs.StoppedContainerStateRequest
	http.DefaultRequest
}

func (h *HttpStopContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpStopContainerRequest) Streamable() bool   { return true }
func (h *HttpStopContainerRequest) HttpPath() string {
	return http.Inline("/container/:id/stopped", string(h.Id))
}
func (h *HttpStopContainerRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.StoppedContainerStateRequest{id}, nil
	}
}

type HttpRestartContainerRequest struct {
	cjobs.RestartContainerRequest
	http.DefaultRequest
}

func (h *HttpRestartContainerRequest) HttpMethod() string { return "POST" }
func (h *HttpRestartContainerRequest) Streamable() bool   { return true }
func (h *HttpRestartContainerRequest) HttpPath() string {
	return http.Inline("/container/:id/restart", string(h.Id))
}
func (h *HttpRestartContainerRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.RestartContainerRequest{id}, nil
	}
}

type HttpBuildImageRequest cjobs.BuildImageRequest

func (h *HttpBuildImageRequest) HttpMethod() string { return "POST" }
func (h *HttpBuildImageRequest) HttpPath() string   { return "/build-image" }
func (h *HttpBuildImageRequest) Streamable() bool   { return true }
func (h *HttpBuildImageRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		data := &cjobs.BuildImageRequest{}
		if r.Body != nil {
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		data.Name = context.Id.String()
		if err := data.Check(); err != nil {
			return nil, err
		}

		return data, nil
	}
}

type HttpPutEnvironmentRequest struct {
	cjobs.PutEnvironmentRequest
	http.DefaultRequest
}

func (h *HttpPutEnvironmentRequest) HttpMethod() string { return "PUT" }
func (h *HttpPutEnvironmentRequest) HttpPath() string {
	return http.Inline("/environment/:id", string(h.Id))
}
func (h *HttpPutEnvironmentRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}

		data := containers.EnvironmentDescription{}
		if r.Body != nil {
			dec := json.NewDecoder(limitedBodyReader(r))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		if err := data.Check(); err != nil {
			return nil, err
		}
		data.Id = id

		return &cjobs.PutEnvironmentRequest{data}, nil
	}
}

type HttpPatchEnvironmentRequest struct {
	cjobs.PatchEnvironmentRequest
	http.DefaultRequest
}

func (h *HttpPatchEnvironmentRequest) HttpMethod() string { return "PATCH" }
func (h *HttpPatchEnvironmentRequest) HttpPath() string {
	return http.Inline("/environment/:id", string(h.Id))
}
func (h *HttpPatchEnvironmentRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}

		data := containers.EnvironmentDescription{}
		if r.Body != nil {
			dec := json.NewDecoder(limitedBodyReader(r))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		if err := data.Check(); err != nil {
			return nil, err
		}
		data.Id = id

		return &cjobs.PatchEnvironmentRequest{data}, nil
	}
}

type HttpContentRequest struct {
	cjobs.ContentRequest
	http.DefaultRequest
}

func (h *HttpContentRequest) HttpMethod() string { return "GET" }
func (h *HttpContentRequest) HttpPath() string {
	var base string
	switch h.Type {
	case cjobs.ContentTypeEnvironment:
		base = "/environment/:id"
	default:
		base = "/content/:id"
	}
	if h.Subpath != "" {
		return base + "/" + h.Subpath
	}
	return http.Inline(base, h.ContentRequest.Locator)
}
func (h *HttpContentRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		if r.PathParam("id") == "" {
			return nil, errors.New("You must specify the location of the content you want to access")
		}
		contentType := r.URL.Query().Get("type")
		if contentType == "" {
			contentType = h.Type
		}
		if contentType == "" {
			return nil, errors.New("You must specify the type of the content you want to access")
		}

		return &cjobs.ContentRequest{
			contentType,
			r.PathParam("id"),
			r.PathParam("*"),
		}, nil
	}
}

type HttpLinkContainersRequest struct {
	cjobs.LinkContainersRequest
	http.DefaultRequest
}

func (h *HttpLinkContainersRequest) HttpMethod() string { return "POST" }
func (h *HttpLinkContainersRequest) HttpPath() string   { return "/containers/links" }
func (h *HttpLinkContainersRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (interface{}, error) {
		data := &containers.ContainerLinks{}
		if r.Body != nil {
			dec := json.NewDecoder(limitedBodyReader(r))
			if err := dec.Decode(data); err != nil && err != io.EOF {
				return nil, err
			}
		}

		if err := data.Check(); err != nil {
			return nil, err
		}

		return &cjobs.LinkContainersRequest{ContainerLinks: data}, nil
	}
}
