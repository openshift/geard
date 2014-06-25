package jobs

import (
	"fmt"
	jobs "github.com/openshift/geard/jobs"
	"github.com/openshift/geard/router"
	"log"
)

type AddRouteRequest struct {
	Frontend          string
	FrontendPath string
	BackendPath  string
	Protocols    []string
	Endpoints      []router.Endpoint
}

type CreateFrontendRequest struct {
	Frontend   string
	Alias string
}

type AddAliasRequest struct {
	Frontend   string
	Alias string
}

type DeleteFrontendRequest struct {
	Frontend string
}

type DeleteRouteRequest struct {
	Frontend      string
	EndpointId string
}

type GetRoutesRequest struct {
	Frontend string
}

func (j AddRouteRequest) Execute(resp jobs.Response) {
	router.AddRoute(j.Frontend, j.FrontendPath, j.BackendPath, j.Protocols, j.Endpoints)
	resp.Success(jobs.ResponseOk)
}

func (j DeleteRouteRequest) Execute(resp jobs.Response) {
	// TODO
	resp.Success(jobs.ResponseOk)
}

func (j CreateFrontendRequest) Execute(resp jobs.Response) {
	value, ok := router.GlobalRoutes[j.Frontend]
	if !ok {
		router.CreateFrontend(j.Frontend, j.Alias)
	} else {
		log.Printf("Error : Frontend %s already exists.", value.Name)
	}
	resp.Success(jobs.ResponseOk)
}

func (j AddAliasRequest) Execute(resp jobs.Response) {
	value, ok := router.GlobalRoutes[j.Frontend]
	if ok {
		router.AddAlias(j.Alias, value.Name)
	} else {
		router.CreateFrontend(j.Frontend, j.Alias)
	}
	resp.Success(jobs.ResponseOk)
}

func (j DeleteFrontendRequest) Execute(resp jobs.Response) {
	router.DeleteFrontend(j.Frontend)
	resp.Success(jobs.ResponseOk)
}

func (j GetRoutesRequest) Execute(resp jobs.Response) {
	// router.ReadRoutes()
	var out string
	if j.Frontend == "*" {
		out = router.PrintRoutes()
	} else {
		out = router.PrintFrontendRoutes(j.Frontend)
	}
	fmt.Println(out)
	resp.SuccessWithData(jobs.ResponseOk, &out)
	// fmt.Fprintf(w, out)
}
