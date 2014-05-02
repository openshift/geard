package http

import (
	"encoding/json"
	"errors"
	"github.com/openshift/geard/containers"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/utils"
	"github.com/openshift/go-json-rest"
	"io"
	"regexp"
)

type DefaultRequest struct{}

type HttpRunContainerRequest struct {
	cjobs.RunContainerRequest
	DefaultRequest
}

func (h *HttpRunContainerRequest) HttpMethod() string { return "POST" }
func (h *HttpRunContainerRequest) HttpPath() string   { return "/jobs" }
func (h *HttpRunContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
	DefaultRequest
}

func (h *HttpInstallContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpInstallContainerRequest) HttpPath() string   { return Inline("/container/:id", string(h.Id)) }
func (h *HttpInstallContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
	DefaultRequest
}

func (h *HttpDeleteContainerRequest) HttpMethod() string { return "DELETE" }
func (h *HttpDeleteContainerRequest) HttpPath() string   { return Inline("/container/:id", string(h.Id)) }
func (h *HttpDeleteContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.DeleteContainerRequest{Id: id}, nil
	}
}

type HttpListContainersRequest struct {
	cjobs.ListContainersRequest
	DefaultRequest
}

func (h *HttpListContainersRequest) HttpMethod() string { return "GET" }
func (h *HttpListContainersRequest) HttpPath() string   { return "/containers" }
func (h *HttpListContainersRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		return &cjobs.ListContainersRequest{}, nil
	}
}

type HttpListBuildsRequest cjobs.ListBuildsRequest

func (h *HttpListBuildsRequest) HttpMethod() string { return "GET" }
func (h *HttpListBuildsRequest) HttpPath() string   { return "/builds" }
func (h *HttpListBuildsRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		return &cjobs.ListBuildsRequest{}, nil
	}
}

type HttpListImagesRequest cjobs.ListImagesRequest

func (h *HttpListImagesRequest) HttpMethod() string { return "GET" }
func (h *HttpListImagesRequest) HttpPath() string   { return "/images" }
func (h *HttpListImagesRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		return &cjobs.ListImagesRequest{conf.Docker.Socket}, nil
	}
}

type HttpContainerLogRequest cjobs.ContainerLogRequest

func (h *HttpContainerLogRequest) HttpMethod() string { return "GET" }
func (h *HttpContainerLogRequest) HttpPath() string   { return Inline("/container/:id/log", string(h.Id)) }
func (h *HttpContainerLogRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
	DefaultRequest
}

func (h *HttpContainerStatusRequest) HttpMethod() string { return "GET" }
func (h *HttpContainerStatusRequest) HttpPath() string {
	return Inline("/container/:id/status", string(h.Id))
}
func (h *HttpContainerStatusRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
	return Inline("/container/:id/ports", string(h.Id))
}
func (h *HttpListContainerPortsRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
	DefaultRequest
}

func (h *HttpStartContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpStartContainerRequest) HttpPath() string {
	return Inline("/container/:id/started", string(h.Id))
}
func (h *HttpStartContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.StartedContainerStateRequest{id}, nil
	}
}

type HttpStopContainerRequest struct {
	cjobs.StoppedContainerStateRequest
	DefaultRequest
}

func (h *HttpStopContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpStopContainerRequest) HttpPath() string {
	return Inline("/container/:id/stopped", string(h.Id))
}
func (h *HttpStopContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		return &cjobs.StoppedContainerStateRequest{id}, nil
	}
}

type HttpRestartContainerRequest struct {
	cjobs.RestartContainerRequest
	DefaultRequest
}

func (h *HttpRestartContainerRequest) HttpMethod() string { return "POST" }
func (h *HttpRestartContainerRequest) HttpPath() string {
	return Inline("/container/:id/restart", string(h.Id))
}
func (h *HttpRestartContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
func (h *HttpBuildImageRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		data := cjobs.ExtendedBuildImageData{}
		if r.Body != nil {
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		data.Name = context.Id.String()
		if err := data.Check(); err != nil {
			return nil, err
		}

		return &cjobs.BuildImageRequest{
			&data,
		}, nil
	}
}

type HttpPutEnvironmentRequest struct {
	cjobs.PutEnvironmentRequest
	DefaultRequest
}

func (h *HttpPutEnvironmentRequest) HttpMethod() string { return "PUT" }
func (h *HttpPutEnvironmentRequest) HttpPath() string   { return Inline("/environment/:id", string(h.Id)) }
func (h *HttpPutEnvironmentRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
	DefaultRequest
}

func (h *HttpPatchEnvironmentRequest) HttpMethod() string { return "PATCH" }
func (h *HttpPatchEnvironmentRequest) HttpPath() string {
	return Inline("/environment/:id", string(h.Id))
}
func (h *HttpPatchEnvironmentRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
	DefaultRequest
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
	return Inline(base, h.ContentRequest.Locator)
}
func (h *HttpContentRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
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
	DefaultRequest
}

func (h *HttpLinkContainersRequest) HttpMethod() string { return "POST" }
func (h *HttpLinkContainersRequest) HttpPath() string   { return "/containers/links" }
func (h *HttpLinkContainersRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		data := &cjobs.ContainerLinks{}
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
