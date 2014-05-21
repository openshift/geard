package jobs

import (
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/jobs"
)

// Return a job extension that casts requests directly to jobs
// TODO: Move implementation out of request object and into a
//   specific package
func NewSshExtension() jobs.JobExtension {
	return &jobs.JobInitializer{
		Extension: jobs.JobExtensionFunc(sharesImplementation),
		Func:      initSsh,
	}
}

func sharesImplementation(request interface{}) (jobs.Job, error) {
	if job, ok := request.(jobs.Job); ok {
		return job, nil
	}
	return nil, jobs.ErrNoJobForRequest
}

// All git jobs depend on these invariants.
// TODO: refactor jobs to take systemd and config
//   as injected dependencies
func initSsh() error {
	if err := config.HasRequiredDirectories(); err != nil {
		return err
	}
	return nil
}
