package http

import (
	"encoding/json"
	"io"

        "fmt"
	"github.com/openshift/geard/router/http/remote"
	"github.com/openshift/geard/jobs"
	rjobs "github.com/openshift/geard/router/jobs"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/http/client"
	"github.com/openshift/go-json-rest"
)

type HttpExtension struct{}

func (h *HttpExtension) Routes() http.ExtensionMap {
	return http.ExtensionMap{
		&remote.HttpRouterCreateFrontendRequest{}: HandleRouterCreateFrontendRequest,
		&remote.HttpRouterCreateRouteRequest{}: HandleRouterCreateRouteRequest,
		&remote.HttpRouterAddAliasRequest{}: HandleRouterAddAliasRequest,
		&remote.HttpRouterDeleteFrontendRequest{}: HandleRouterDeleteFrontendRequest,
		&remote.HttpRouterDeleteRouteRequest{}: HandleRouterDeleteRouteRequest,
		&remote.HttpRouterGetRoutesRequest{}: HandleRouterGetRoutesRequest,
	}
}

func (h *HttpExtension) HttpJobFor(job interface{}) (exc client.RemoteExecutable, err error) {
	return remote.HttpJobFor(job)
}

func HandleRouterCreateFrontendRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
		frontendname := r.PathParam("id")
		var data rjobs.CreateFrontendRequest
		alias := ""
		if r.Body != nil {
			dec := json.NewDecoder(io.LimitReader(r.Body, 100*1024))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				fmt.Println("Error decoding json body - %s", err.Error())
				return nil, err
			}
			alias = data.Alias
		}
		return &rjobs.CreateFrontendRequest{frontendname, alias}, nil
}

func HandleRouterCreateRouteRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
		frontendname := r.PathParam("id")
		var data rjobs.AddRouteRequest
		if r.Body != nil {
			dec := json.NewDecoder(io.LimitReader(r.Body, 100*1024))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				fmt.Println("Error decoding json body - %s", err.Error())
				return nil, err
			}
		} else {
			// error
			return nil, jobs.SimpleError{jobs.ResponseInvalidRequest, fmt.Sprintf("Insufficient data to create route.")}
		}
		data.Frontend = frontendname
		return &rjobs.AddRouteRequest{data.Frontend, data.FrontendPath, data.BackendPath, data.Protocols, data.Endpoints}, nil
}

func HandleRouterDeleteFrontendRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
		frontendname := r.PathParam("id")
		return &rjobs.DeleteFrontendRequest{frontendname}, nil
}

func HandleRouterDeleteRouteRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
		frontendname := r.PathParam("id")
		epid := r.PathParam("endpointId")
		return &rjobs.DeleteRouteRequest{frontendname, epid}, nil
}

func HandleRouterGetRoutesRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
		frontendname := r.PathParam("id")
		return &rjobs.GetRoutesRequest{frontendname}, nil
}

func HandleRouterAddAliasRequest(conf *http.HttpConfiguration, context *http.HttpContext, r *rest.Request) (interface{}, error) {
		frontendname := r.PathParam("id")
		var data rjobs.AddAliasRequest
		if r.Body != nil {
			dec := json.NewDecoder(io.LimitReader(r.Body, 100*1024))
			if err := dec.Decode(&data); err != nil && err != io.EOF {
				fmt.Println("Error decoding json body - %s", err.Error())
				return nil, err
			}
		} else {
			// error
			return nil, jobs.SimpleError{jobs.ResponseInvalidRequest, fmt.Sprintf("Insufficient data to add alias.")}
		}
		data.Frontend = frontendname
		return &rjobs.AddAliasRequest{frontendname, data.Alias}, nil
}
