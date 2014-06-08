package jobs

import (
	"errors"
	"regexp"

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

const ContentTypeGitArchive = "gitarchive"

type GitCommitRef string

const EmptyGitCommitRef = GitCommitRef("")
const InvalidGitCommitRef = GitCommitRef("")

var allowedGitCommitRef = regexp.MustCompile("\\A[a-zA-Z0-9_\\-]+\\z")

func NewGitCommitRef(s string) (GitCommitRef, error) {
	switch {
	case s == "":
		return EmptyGitCommitRef, nil
	case !allowedGitCommitRef.MatchString(s):
		return InvalidGitCommitRef, errors.New("Git ref must match " + allowedGitCommitRef.String())
	}
	return GitCommitRef(s), nil
}

type GitArchiveContentRequest struct {
	RepositoryId git.RepoIdentifier
	Ref          GitCommitRef
}
