package linux

import (
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/jobs"
	rjobs "github.com/openshift/geard/router/jobs"
	"github.com/openshift/geard/systemd"
)

// Return a job extension that casts requests directly to jobs
// TODO: Move implementation out of request object and into a
//   specific package
func NewRouterExtension() jobs.JobExtension {
	return &jobs.JobInitializer{
		Extension: jobs.JobExtensionFunc(sharesImplementation),
		Func:      initRouter,
	}
}

func sharesImplementation(request interface{}) (jobs.Job, error) {
	switch r := request.(type) {
		case *rjobs.AddRouteRequest :
		        return r, nil
		case *rjobs.CreateFrontendRequest :
			return r, nil
		case *rjobs.AddAliasRequest :
			return r, nil
		case *rjobs.DeleteFrontendRequest:
			return r, nil
		case *rjobs.DeleteRouteRequest :
			return r, nil
		case *rjobs.GetRoutesRequest :
			return r, nil
	}
	return nil, jobs.ErrNoJobForRequest
}

// TODO: refactor jobs to take systemd and config
//   as injected dependencies
func initRouter() error {
	if err := config.HasRequiredDirectories(); err != nil {
		return err
	}
	if err := systemd.Start(); err != nil {
		return err
	}
	if err := InitializeServices(); err != nil {
		return err
	}
	return nil
}
