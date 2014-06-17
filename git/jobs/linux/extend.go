// +build linux

package linux

import (
	"log"

	"github.com/openshift/geard/config"
	gjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/systemd"
)

// Return a job extension that casts requests directly to jobs
// TODO: Move implementation out of request object and into a
//   specific package
func NewGitExtension() jobs.JobExtension {
	return &jobs.JobInitializer{
		Extension: jobs.JobExtensionFunc(implementsJob),
		Func:      initGit,
	}
}

func implementsJob(request interface{}) (jobs.Job, error) {
	switch r := request.(type) {
	case *gjobs.CreateRepositoryRequest:
		conn, err := systemd.NewConnection()
		if err != nil {
			log.Print("create_repository:", err)
			return nil, err
		}
		return &createRepository{r, conn}, nil
	case *gjobs.GitArchiveContentRequest:
		return &archiveRepository{r}, nil
	}
	return nil, jobs.ErrNoJobForRequest
}

// All git jobs depend on these invariants.
// TODO: refactor jobs to take systemd and config
//   as injected dependencies
func initGit() error {
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
