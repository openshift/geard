package http

import (
	"encoding/json"
	"io"

	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/containers/http/remote"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/http/client"
	"github.com/openshift/go-json-rest"
)

type HttpExtension struct{}

func (h *HttpExtension) Routes() http.ExtensionMap {
	return http.ExtensionMap{
		&remote.HttpRunContainerRequest{}:       HandleRunContainerRequest,
		&remote.HttpInstallContainerRequest{}:   HandleInstallContainerRequest,
		&remote.HttpDeleteContainerRequest{}:    HandleDeleteContainerRequest,
		&remote.HttpContainerLogRequest{}:       HandleContainerLogRequest,
		&remote.HttpContainerStatusRequest{}:    HandleContainerStatusRequest,
		&remote.HttpListContainerPortsRequest{}: HandleContainerPortsRequest,
		&remote.HttpPurgeContainersRequest{}:    HandlePurgeContainersRequest,

		&remote.HttpStartContainerRequest{}:   HandleStartContainerRequest,
		&remote.HttpStopContainerRequest{}:    HandleStopContainerRequest,
		&remote.HttpRestartContainerRequest{}: HandleRestartContainerRequest,

		&remote.HttpLinkContainersRequest{}: HandleLinkContainersRequest,

		&remote.HttpListContainersRequest{}: HandleListContainersRequest,
		&remote.HttpListImagesRequest{}:     HandleListImagesRequest,
		&remote.HttpListBuildsRequest{}:     HandleListBuildsRequest,

		&remote.HttpBuildImageRequest{}: HandleBuildImageRequest,

		&remote.HttpGetEnvironmentRequest{}:   HandleGetEnvironmentRequest,
		&remote.HttpPatchEnvironmentRequest{}: HandlePatchEnvironmentRequest,
		&remote.HttpPutEnvironmentRequest{}:   HandlePutEnvironmentRequest,
	}
}
func (h *HttpExtension) HttpJobFor(job interface{}) (exc client.RemoteExecutable, err error) {
	return remote.HttpJobFor(job)
}

func HandleRunContainerRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	data := &cjobs.RunContainerRequest{}
	if err := decodeBody(r, data); err != nil {
		return nil, err
	}
	data.Name = context.Id.String()
	if err := data.Check(); err != nil {
		return nil, err
	}
	return data, nil
}

func HandleInstallContainerRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	data := &cjobs.InstallContainerRequest{}
	if err := decodeBody(r, data); err != nil {
		return nil, err
	}
	data.Id = id
	data.RequestIdentifier = context.Id

	if err := data.Check(); err != nil {
		return nil, err
	}
	return data, nil
}

func HandleDeleteContainerRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	return &cjobs.DeleteContainerRequest{Id: id}, nil
}

func HandlePurgeContainersRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	return &cjobs.PurgeContainersRequest{}, nil
}

func HandleListContainersRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	return &cjobs.ListContainersRequest{}, nil
}

func HandleListBuildsRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	return &cjobs.ListBuildsRequest{}, nil
}

func HandleListImagesRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	return &cjobs.ListImagesRequest{conf.Docker.Socket}, nil
}

func HandleContainerLogRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	return &cjobs.ContainerLogRequest{id}, nil
}

func HandleContainerStatusRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	return &cjobs.ContainerStatusRequest{Id: id}, nil
}

func HandleContainerPortsRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	return &cjobs.ContainerPortsRequest{id}, nil
}

func HandleStartContainerRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	return &cjobs.StartedContainerStateRequest{id}, nil
}

func HandleStopContainerRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	data := &cjobs.StoppedContainerStateRequest{}
	if err := decodeAndCheck(r, data); err != nil {
		return nil, err
	}
	data.Id = id
	return data, nil
}

func HandleRestartContainerRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	return &cjobs.RestartContainerRequest{id}, nil
}

func HandleBuildImageRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	data := &cjobs.BuildImageRequest{}
	if err := decodeBody(r, data); err != nil {
		return nil, err
	}
	data.Name = context.Id.String()
	if err := data.Check(); err != nil {
		return nil, err
	}
	return data, nil
}

func HandleGetEnvironmentRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	return &cjobs.GetEnvironmentRequest{id}, nil
}

func HandlePutEnvironmentRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	data := containers.EnvironmentDescription{}
	if err := decodeAndCheck(r, &data); err != nil {
		return nil, err
	}
	data.Id = id
	return &cjobs.PutEnvironmentRequest{data}, nil
}

func HandlePatchEnvironmentRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	id, errg := containers.NewIdentifier(r.PathParam("id"))
	if errg != nil {
		return nil, errg
	}
	data := containers.EnvironmentDescription{}
	if err := decodeAndCheck(r, &data); err != nil {
		return nil, err
	}
	data.Id = id
	return &cjobs.PatchEnvironmentRequest{data}, nil
}

func HandleLinkContainersRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
	data := &containers.ContainerLinks{}
	if err := decodeAndCheck(r, data); err != nil {
		return nil, err
	}
	return &cjobs.LinkContainersRequest{ContainerLinks: data}, nil
}

func limitedBodyReader(r *rest.Request) io.Reader {
	return io.LimitReader(r.Body, 100*1024)
}

type check interface {
	Check() error
}

func decodeBody(r *rest.Request, into interface{}) error {
	if r.Body != nil {
		dec := json.NewDecoder(limitedBodyReader(r))
		if err := dec.Decode(into); err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}

func decodeAndCheck(r *rest.Request, into interface{}) error {
	if err := decodeBody(r, into); err != nil {
		return err
	}
	if c, ok := into.(check); ok {
		if err := c.Check(); err != nil {
			return err
		}
	}
	return nil
}
