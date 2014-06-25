package remote

import (
	rjobs "github.com/openshift/geard/router/jobs"
	"github.com/openshift/geard/http/client"
)

func HttpJobFor(job interface{}) (exc client.RemoteExecutable, err error) {
	switch j := job.(type) {
	case *rjobs.CreateFrontendRequest:
		exc = &HttpRouterCreateFrontendRequest{CreateFrontendRequest: *j}
	case *rjobs.AddRouteRequest:
		exc = &HttpRouterCreateRouteRequest{AddRouteRequest: *j}
	case *rjobs.AddAliasRequest:
		exc = &HttpRouterAddAliasRequest{AddAliasRequest: *j}
	case *rjobs.DeleteFrontendRequest:
		exc = &HttpRouterDeleteFrontendRequest{DeleteFrontendRequest: *j}
	case *rjobs.GetRoutesRequest:
		exc = &HttpRouterGetRoutesRequest{GetRoutesRequest: *j}
	}
	return
}


type HttpRouterCreateFrontendRequest struct {
	rjobs.CreateFrontendRequest
	client.DefaultRequest
}

type HttpRouterAddAliasRequest struct {
	rjobs.AddAliasRequest
	client.DefaultRequest
}

type HttpRouterDeleteFrontendRequest struct {
	rjobs.DeleteFrontendRequest
	client.DefaultRequest
}

type HttpRouterDeleteRouteRequest struct {
	rjobs.DeleteRouteRequest
	client.DefaultRequest
}

type HttpRouterCreateRouteRequest struct {
	rjobs.AddRouteRequest
	client.DefaultRequest
}

type HttpRouterGetRoutesRequest struct {
	rjobs.GetRoutesRequest
	client.DefaultRequest
}

func (h *HttpRouterCreateFrontendRequest) HttpMethod() string { return "POST" }
func (h *HttpRouterCreateFrontendRequest) HttpPath() string {
	return client.Inline("/frontend/:id", string(h.Frontend))
}

func (h *HttpRouterCreateRouteRequest) HttpMethod() string { return "POST" }
func (h *HttpRouterCreateRouteRequest) HttpPath() string {
	return client.Inline("/frontend/:id/routes", string(h.Frontend))
}

func (h *HttpRouterDeleteFrontendRequest) HttpMethod() string { return "DELETE" }
func (h *HttpRouterDeleteFrontendRequest) HttpPath() string {
	return client.Inline("/frontend/:id", string(h.Frontend))
}

func (h *HttpRouterDeleteRouteRequest) HttpMethod() string { return "DELETE" }
func (h *HttpRouterDeleteRouteRequest) HttpPath() string {
	return client.Inline("/frontend/:id/routes/:endpointId", string(h.Frontend), string(h.EndpointId))
}

func (h *HttpRouterGetRoutesRequest) HttpMethod() string { return "GET" }
func (h *HttpRouterGetRoutesRequest) HttpPath() string {
	return client.Inline("/frontend/:id/routes", string(h.Frontend))
}

func (h *HttpRouterAddAliasRequest) HttpMethod() string { return "POST" }
func (h *HttpRouterAddAliasRequest) HttpPath() string {
	return client.Inline("/frontend/:id/aliases", string(h.Frontend))
}

