package http

import (
	"fmt"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/git"
	gitjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/go-json-rest"
)

func Routes() []http.HttpJobHandler {
	return []http.HttpJobHandler{
		&HttpCreateRepositoryRequest{},
		&httpGitArchiveContentRequest{
			GitArchiveContentRequest: gitjobs.GitArchiveContentRequest{Ref: "*"},
		},
	}
}

type HttpCreateRepositoryRequest struct {
	gitjobs.CreateRepositoryRequest
	http.DefaultRequest
}

func (h *HttpCreateRepositoryRequest) HttpMethod() string { return "PUT" }
func (h *HttpCreateRepositoryRequest) HttpPath() string {
	return http.Inline("/repository/:id", string(h.Id))
}
func (h *HttpCreateRepositoryRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		repositoryId, errg := containers.NewIdentifier(r.PathParam("id"))
		if errg != nil {
			return nil, errg
		}
		// TODO: convert token into a safe clone spec and commit hash
		return &gitjobs.CreateRepositoryRequest{
			git.RepoIdentifier(repositoryId),
			r.URL.Query().Get("source"),
			context.Id,
		}, nil
	}
}

type httpGitArchiveContentRequest struct {
	gitjobs.GitArchiveContentRequest
	http.DefaultRequest
}

func (h *httpGitArchiveContentRequest) HttpMethod() string { return "GET" }
func (h *httpGitArchiveContentRequest) HttpPath() string {
	return http.Inline("/repository/:id/archive/:ref", string(h.RepositoryId), string(h.Ref))
}
func (h *httpGitArchiveContentRequest) Handler(conf *http.HttpConfiguration) http.JobHandler {
	return func(context *jobs.JobContext, r *rest.Request) (jobs.Job, error) {
		repoId, errr := containers.NewIdentifier(r.PathParam("id"))
		if errr != nil {
			return nil, jobs.SimpleJobError{jobs.JobResponseInvalidRequest, fmt.Sprintf("Invalid repository identifier: %s", errr.Error())}
		}
		ref, errc := gitjobs.NewGitCommitRef(r.PathParam("ref"))
		if errc != nil {
			return nil, jobs.SimpleJobError{jobs.JobResponseInvalidRequest, fmt.Sprintf("Invalid commit ref: %s", errc.Error())}
		}

		return &gitjobs.GitArchiveContentRequest{
			git.RepoIdentifier(repoId),
			ref,
		}, nil
	}
}
