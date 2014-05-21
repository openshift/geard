package jobs

import (
	"github.com/openshift/geard/git"
	"github.com/openshift/geard/jobs"
)

var (
	ErrRepositoryAlreadyExists = jobs.SimpleError{jobs.ResponseAlreadyExists, "A repository with this identifier already exists."}
	ErrSubscribeToUnit         = jobs.SimpleError{jobs.ResponseError, "Unable to watch for the completion of this action."}
	ErrRepositoryCreateFailed  = jobs.SimpleError{jobs.ResponseError, "Unable to create the repository."}
)

type CreateRepositoryRequest struct {
	Id        git.RepoIdentifier
	CloneUrl  string
	RequestId jobs.RequestIdentifier
}
