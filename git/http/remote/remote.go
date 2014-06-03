package remote

import (
	gitjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/http/client"
	"github.com/openshift/geard/jobs"
)

func HttpJobFor(job interface{}) (exc client.RemoteExecutable, err error) {
	switch j := job.(type) {
	case *gitjobs.CreateRepositoryRequest:
		exc = &HttpCreateRepositoryRequest{CreateRepositoryRequest: *j}
	case *gitjobs.GitArchiveContentRequest:
		exc = &HttpGitArchiveContentRequest{GitArchiveContentRequest: *j}
	default:
		err = jobs.ErrNoJobForRequest
	}
	return
}

type HttpCreateRepositoryRequest struct {
	gitjobs.CreateRepositoryRequest
	client.DefaultRequest
}

func (h *HttpCreateRepositoryRequest) HttpMethod() string { return "PUT" }
func (h *HttpCreateRepositoryRequest) HttpPath() string {
	return client.Inline("/repository/:id", string(h.Id))
}

type HttpGitArchiveContentRequest struct {
	gitjobs.GitArchiveContentRequest
	client.DefaultRequest
}

func (h *HttpGitArchiveContentRequest) HttpMethod() string { return "GET" }
func (h *HttpGitArchiveContentRequest) HttpPath() string {
	return client.Inline("/repository/:id/archive/:ref", string(h.RepositoryId), string(h.Ref))
}
