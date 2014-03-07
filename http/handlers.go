package http

import (
	"encoding/json"
	"errors"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/go-json-rest"
	"io"
)

type DefaultRequest struct{}

type HttpInstallContainerRequest struct {
	jobs.InstallContainerRequest
	DefaultRequest
}

func (h *HttpInstallContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpInstallContainerRequest) HttpPath() string   { return "/container" }
func (h *HttpInstallContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		data := HttpInstallContainerRequest{}
		if r.Body != nil {
			dec := json.NewDecoder(limitedBodyReader(r))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		id, errg := containers.NewIdentifier(token.ResourceLocator())
		if errg != nil {
			return nil, errg
		}
		data.Id = id
		data.RequestIdentifier = reqid
		data.Image = token.ResourceType()

		if err := data.Check(); err != nil {
			return nil, err
		}
		return &data, nil
	}
}

type HttpDeleteContainerRequest struct {
	jobs.DeleteContainerRequest
	DefaultRequest
	Label string
}

func (h *HttpDeleteContainerRequest) HttpMethod() string { return "DELETE" }
func (h *HttpDeleteContainerRequest) HttpPath() string   { return "/container" }
func (h *HttpDeleteContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(token.ResourceLocator())
		if errg != nil {
			return nil, errg
		}
		return &jobs.DeleteContainerRequest{id}, nil
	}
}

type HttpListContainersRequest struct {
	jobs.ListContainersRequest
	DefaultRequest
	Label string
}

func (h *HttpListContainersRequest) HttpMethod() string { return "GET" }
func (h *HttpListContainersRequest) HttpPath() string   { return "/containers" }
func (h *HttpListContainersRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		return &jobs.ListContainersRequest{}, nil
	}
}

type HttpListBuildsRequest jobs.ListBuildsRequest

func (h *HttpListBuildsRequest) HttpMethod() string { return "GET" }
func (h *HttpListBuildsRequest) HttpPath() string   { return "/builds" }
func (h *HttpListBuildsRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		return &jobs.ListBuildsRequest{}, nil
	}
}

type HttpListImagesRequest jobs.ListImagesRequest

func (h *HttpListImagesRequest) HttpMethod() string { return "GET" }
func (h *HttpListImagesRequest) HttpPath() string   { return "/images" }
func (h *HttpListImagesRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		return &jobs.ListImagesRequest{conf.Docker.Socket}, nil
	}
}

type HttpContainerLogRequest jobs.ContainerLogRequest

func (h *HttpContainerLogRequest) HttpMethod() string { return "GET" }
func (h *HttpContainerLogRequest) HttpPath() string   { return "/container/log" }
func (h *HttpContainerLogRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(token.ResourceLocator())
		if errg != nil {
			return nil, errg
		}
		return &jobs.ContainerLogRequest{
			id,
			token.U,
		}, nil
	}
}

type HttpContainerStatusRequest struct {
	jobs.ContainerStatusRequest
	DefaultRequest
}

func (h *HttpContainerStatusRequest) HttpMethod() string { return "GET" }
func (h *HttpContainerStatusRequest) HttpPath() string   { return "/container/status" }
func (h *HttpContainerStatusRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(token.ResourceLocator())
		if errg != nil {
			return nil, errg
		}
		return &jobs.ContainerStatusRequest{Id: id}, nil
	}
}

type HttpListContainerPortsRequest jobs.ContainerPortsRequest

func (h *HttpListContainerPortsRequest) HttpMethod() string { return "GET" }
func (h *HttpListContainerPortsRequest) HttpPath() string   { return "/container/ports" }
func (h *HttpListContainerPortsRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(token.ResourceLocator())
		if errg != nil {
			return nil, errg
		}
		return &jobs.ContainerPortsRequest{
			id,
			token.U,
		}, nil
	}
}

type HttpCreateKeysRequest jobs.CreateKeysRequest

func (h *HttpCreateKeysRequest) HttpMethod() string { return "PUT" }
func (h *HttpCreateKeysRequest) HttpPath() string   { return "/keys" }
func (h *HttpCreateKeysRequest) Streamable() bool   { return false }
func (h *HttpCreateKeysRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		data := jobs.ExtendedCreateKeysData{}
		if r.Body != nil {
			dec := json.NewDecoder(limitedBodyReader(r))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}
		if err := data.Check(); err != nil {
			return nil, err
		}
		return &jobs.CreateKeysRequest{
			token.U,
			&data,
		}, nil
	}
}

type HttpStartContainerRequest struct {
	jobs.StartedContainerStateRequest
	DefaultRequest
}

func (h *HttpStartContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpStartContainerRequest) HttpPath() string   { return "/container/started" }
func (h *HttpStartContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(token.ResourceLocator())
		if errg != nil {
			return nil, errg
		}
		return &jobs.StartedContainerStateRequest{id}, nil
	}
}

type HttpStopContainerRequest struct {
	jobs.StoppedContainerStateRequest
	DefaultRequest
}

func (h *HttpStopContainerRequest) HttpMethod() string { return "PUT" }
func (h *HttpStopContainerRequest) HttpPath() string   { return "/container/stopped" }
func (h *HttpStopContainerRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(token.ResourceLocator())
		if errg != nil {
			return nil, errg
		}
		return &jobs.StoppedContainerStateRequest{id}, nil
	}
}

type HttpBuildImageRequest jobs.BuildImageRequest

func (h *HttpBuildImageRequest) HttpMethod() string { return "PUT" }
func (h *HttpBuildImageRequest) HttpPath() string   { return "/build-image" }
func (h *HttpBuildImageRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		if token.ResourceLocator() == "" {
			return nil, errors.New("You must specify the application source to build")
		}
		if token.ResourceType() == "" {
			return nil, errors.New("You must specify a base image")
		}

		source := token.ResourceLocator() // token.R
		baseImage := token.ResourceType() // token.T
		tag := token.U

		data := jobs.ExtendedBuildImageData{}
		if r.Body != nil {
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}

		return &jobs.BuildImageRequest{
			reqid.String(),
			source,
			baseImage,
			tag,
			&data,
		}, nil
	}
}

type HttpPutEnvironmentRequest struct {
	jobs.PutEnvironmentRequest
	DefaultRequest
}

func (h *HttpPutEnvironmentRequest) HttpMethod() string { return "PUT" }
func (h *HttpPutEnvironmentRequest) HttpPath() string   { return "/environment" }
func (h *HttpPutEnvironmentRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(token.ResourceLocator())
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

		return &jobs.PutEnvironmentRequest{
			&data,
		}, nil
	}
}

type HttpPatchEnvironmentRequest struct {
	jobs.PatchEnvironmentRequest
	DefaultRequest
}

func (h *HttpPatchEnvironmentRequest) HttpMethod() string { return "PATCH" }
func (h *HttpPatchEnvironmentRequest) HttpPath() string   { return "/environment" }
func (h *HttpPatchEnvironmentRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		id, errg := containers.NewIdentifier(token.ResourceLocator())
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

		return &jobs.PatchEnvironmentRequest{
			&data,
		}, nil
	}
}

type HttpContentRequest struct {
	jobs.ContentRequest
	DefaultRequest
}

func (h *HttpContentRequest) HttpMethod() string { return "GET" }
func (h *HttpContentRequest) HttpPath() string {
	var base string
	switch h.Type {
	case jobs.ContentTypeEnvironment:
		base = "/environment"
	default:
		base = "/content"
	}
	if h.Subpath != "" {
		return base + "/" + h.Subpath
	}
	return base
}
func (h *HttpContentRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		if token.ResourceLocator() == "" {
			return nil, errors.New("You must specify the location of the content you want to access")
		}
		contentType := token.ResourceType()
		if contentType == "" {
			contentType = h.Type
		}
		if contentType == "" {
			return nil, errors.New("You must specify the type of the content you want to access")
		}

		return &jobs.ContentRequest{
			contentType,
			token.ResourceLocator(),
			r.PathParam("*"),
		}, nil
	}
}

type HttpLinkContainersRequest jobs.LinkContainersRequest

func (h *HttpLinkContainersRequest) HttpMethod() string { return "PUT" }
func (h *HttpLinkContainersRequest) HttpPath() string   { return "/containers/links" }
func (h *HttpLinkContainersRequest) Handler(conf *HttpConfiguration) JobHandler {
	return func(reqid jobs.RequestIdentifier, token *TokenData, r *rest.Request) (jobs.Job, error) {
		data := jobs.ExtendedLinkContainersData{}
		if r.Body != nil {
			dec := json.NewDecoder(limitedBodyReader(r))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				return nil, err
			}
		}

		if err := data.Check(); err != nil {
			return nil, err
		}

		return &jobs.LinkContainersRequest{
			&data,
		}, nil
	}
}
