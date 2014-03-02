package git

import (
	"fmt"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/http"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/go-json-rest"
)

func Routes() []http.RestRoute {
	return []http.RestRoute{
		http.RestRoute{"PUT", "/token/:token/repository", apiPutRepository},
		http.RestRoute{"GET", "/token/:token/repository/archive/*", apiGetArchive},
	}
}

func apiPutRepository(reqid jobs.RequestIdentifier, token *http.TokenData, w *rest.ResponseWriter, r *rest.Request) (jobs.Job, error) {
	repositoryId, errg := gears.NewIdentifier(token.ResourceLocator())
	if errg != nil {
		return nil, errg
	}
	// TODO: convert token into a safe clone spec and commit hash
	return &CreateRepositoryRequest{
		http.NewHttpJobResponse(w.ResponseWriter, false),
		jobs.JobRequest{reqid},
		repositoryId,
		"ccoleman/githost",
		token.ResourceType(),
	}, nil
}

func apiGetArchive(reqid jobs.RequestIdentifier, token *http.TokenData, w *rest.ResponseWriter, r *rest.Request) (jobs.Job, error) {
	repoId, errr := gears.NewIdentifier(token.ResourceLocator())
	if errr != nil {
		return nil, jobs.SimpleJobError{jobs.JobResponseInvalidRequest, fmt.Sprintf("Invalid repository identifier: %s", errr.Error())}
	}
	ref, errc := NewGitCommitRef(r.PathParam("*"))
	if errc != nil {
		return nil, jobs.SimpleJobError{jobs.JobResponseInvalidRequest, fmt.Sprintf("Invalid commit ref: %s", errc.Error())}
	}

	return &GitArchiveContentRequest{
		http.NewHttpJobResponse(w.ResponseWriter, false),
		jobs.JobRequest{reqid},
		repoId,
		ref,
	}, nil
}
