package jobs

import (
	// "fmt"
	// "github.com/openshift/geard/config"
	jobs "github.com/openshift/geard/jobs"
	"github.com/openshift/geard/router"
	"github.com/openshift/geard/utils"
	// "io"
	// "log"
	// "os"
	// "os/exec"
	// "os/user"
	// "path/filepath"
	// "strconv"
	// "time"
)

// var (
// 	ErrRepositoryAlreadyExists = jobs.SimpleJobError{jobs.JobResponseAlreadyExists, "A repository with this identifier already exists."}
// 	ErrSubscribeToUnit         = jobs.SimpleJobError{jobs.JobResponseError, "Unable to watch for the completion of this action."}
// 	ErrRepositoryCreateFailed  = jobs.SimpleJobError{jobs.JobResponseError, "Unable to create the repository."}
// )

type UpdateFrontendRequest struct {
	Frontends []FrontendDescription
	Backends  []BackendDescription
}

type backendError struct {
	Id    router.Identifier
	Error error
}

func (j UpdateFrontendRequest) Execute(resp jobs.JobResponse) {
	// detach empty frontends
	for _, frontend := range j.Frontends {
		if frontend.BackendId == "" {
			frontend.Remove()
		}
	}

	errs := []backendError{}
	for _, backend := range j.Backends {
		if err := utils.WriteToPathExclusive(backend.Id.BackendPathFor(), 0554, backend); err != nil {
			errs = append(errs, backendError{backend.Id, err})
		}
	}
	if len(errs) != 0 {
		log.Printf("Unable to persist some backends: %+v", errs)
		resp.Failure(ErrBackendWriteFailed)
		return
	}
	resp.Success(jobs.JobResponseOk)
}
