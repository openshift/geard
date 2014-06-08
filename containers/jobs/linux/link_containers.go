package linux

import (
	. "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
)

type linkContainers struct {
	*LinkContainersRequest
}

func (j *linkContainers) Execute(resp jobs.Response) {
	for i := range j.Links {
		if errw := j.Links[i].NetworkLinks.Write(j.Links[i].Id.NetworkLinksPathFor(), false); errw != nil {
			resp.Failure(ErrLinkContainersFailed)
			return
		}
	}

	resp.Success(jobs.ResponseOk)
}
