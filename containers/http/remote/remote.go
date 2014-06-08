package remote

import (
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/http/client"
	"github.com/openshift/geard/jobs"
)

func HttpJobFor(job interface{}) (exc client.RemoteExecutable, err error) {
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
	case *cjobs.PurgeContainersRequest:
		exc = &HttpPurgeContainersRequest{PurgeContainersRequest: *j}
	default:
		err = jobs.ErrNoJobForRequest
	}
	return
}

type HttpRunContainerRequest struct {
	cjobs.RunContainerRequest
	client.DefaultRequest
}

func (h *HttpRunContainerRequest) HttpMethod() string { return "POST" }
func (h *HttpRunContainerRequest) HttpPath() string   { return "/jobs" }

type HttpInstallContainerRequest struct {
	cjobs.InstallContainerRequest
	client.DefaultRequest
}

func (h *HttpInstallContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpInstallContainerRequest) HttpPath() string {
	return client.Inline("/container/:id", string(h.Id))
}

type HttpDeleteContainerRequest struct {
	cjobs.DeleteContainerRequest
	client.DefaultRequest
}

func (h *HttpDeleteContainerRequest) HttpMethod() string { return "DELETE" }
func (h *HttpDeleteContainerRequest) HttpPath() string {
	return client.Inline("/container/:id", string(h.Id))
}

type HttpListContainersRequest struct {
	cjobs.ListContainersRequest
	client.DefaultRequest
}

func (h *HttpListContainersRequest) HttpMethod() string { return "GET" }
func (h *HttpListContainersRequest) HttpPath() string   { return "/containers" }

type HttpListBuildsRequest struct {
	cjobs.ListBuildsRequest
	client.DefaultRequest
}

func (h *HttpListBuildsRequest) HttpMethod() string { return "GET" }
func (h *HttpListBuildsRequest) HttpPath() string   { return "/builds" }

type HttpListImagesRequest struct {
	cjobs.ListImagesRequest
	client.DefaultRequest
}

func (h *HttpListImagesRequest) HttpMethod() string { return "GET" }
func (h *HttpListImagesRequest) HttpPath() string   { return "/images" }

type HttpContainerLogRequest struct {
	cjobs.ContainerLogRequest
	client.DefaultRequest
}

func (h *HttpContainerLogRequest) HttpMethod() string { return "GET" }
func (h *HttpContainerLogRequest) HttpPath() string {
	return client.Inline("/container/:id/log", string(h.Id))
}

type HttpContainerStatusRequest struct {
	cjobs.ContainerStatusRequest
	client.DefaultRequest
}

func (h *HttpContainerStatusRequest) HttpMethod() string { return "GET" }
func (h *HttpContainerStatusRequest) HttpPath() string {
	return client.Inline("/container/:id/status", string(h.Id))
}

type HttpListContainerPortsRequest struct {
	cjobs.ContainerPortsRequest
	client.DefaultRequest
}

func (h *HttpListContainerPortsRequest) HttpMethod() string { return "GET" }
func (h *HttpListContainerPortsRequest) HttpPath() string {
	return client.Inline("/container/:id/ports", string(h.Id))
}

type HttpStartContainerRequest struct {
	cjobs.StartedContainerStateRequest
	client.DefaultRequest
}

func (h *HttpStartContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpStartContainerRequest) HttpPath() string {
	return client.Inline("/container/:id/started", string(h.Id))
}

type HttpStopContainerRequest struct {
	cjobs.StoppedContainerStateRequest
	client.DefaultRequest
}

func (h *HttpStopContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpStopContainerRequest) HttpPath() string {
	return client.Inline("/container/:id/stopped", string(h.Id))
}

type HttpRestartContainerRequest struct {
	cjobs.RestartContainerRequest
	client.DefaultRequest
}

func (h *HttpRestartContainerRequest) HttpMethod() string { return "POST" }
func (h *HttpRestartContainerRequest) HttpPath() string {
	return client.Inline("/container/:id/restart", string(h.Id))
}

type HttpBuildImageRequest struct {
	cjobs.BuildImageRequest
	client.DefaultRequest
}

func (h *HttpBuildImageRequest) HttpMethod() string { return "POST" }
func (h *HttpBuildImageRequest) HttpPath() string   { return "/build-image" }

type HttpGetEnvironmentRequest struct {
	cjobs.GetEnvironmentRequest
	client.DefaultRequest
}

func (h *HttpGetEnvironmentRequest) HttpMethod() string { return "GET" }
func (h *HttpGetEnvironmentRequest) HttpPath() string {
	return client.Inline("/environment/:id", string(h.Id))
}

type HttpPutEnvironmentRequest struct {
	cjobs.PutEnvironmentRequest
	client.DefaultRequest
}

func (h *HttpPutEnvironmentRequest) HttpMethod() string { return "PUT" }
func (h *HttpPutEnvironmentRequest) HttpPath() string {
	return client.Inline("/environment/:id", string(h.Id))
}

type HttpPatchEnvironmentRequest struct {
	cjobs.PatchEnvironmentRequest
	client.DefaultRequest
}

func (h *HttpPatchEnvironmentRequest) HttpMethod() string { return "PATCH" }
func (h *HttpPatchEnvironmentRequest) HttpPath() string {
	return client.Inline("/environment/:id", string(h.Id))
}

type HttpContentRequest struct {
	cjobs.ContentRequest
	client.DefaultRequest
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
	return client.Inline(base, h.ContentRequest.Locator)
}

type HttpLinkContainersRequest struct {
	cjobs.LinkContainersRequest
	client.DefaultRequest
}

func (h *HttpLinkContainersRequest) HttpMethod() string { return "POST" }
func (h *HttpLinkContainersRequest) HttpPath() string   { return "/containers/links" }

type HttpPurgeContainersRequest struct {
	cjobs.PurgeContainersRequest
	client.DefaultRequest
}

func (h *HttpPurgeContainersRequest) HttpMethod() string { return "DELETE" }
func (h *HttpPurgeContainersRequest) HttpPath() string   { return "/containers" }
