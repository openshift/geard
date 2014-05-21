// +build linux

package jobs

import (
	"github.com/openshift/geard/jobs"
)

func (j *LinkContainersRequest) Execute(resp jobs.Response) {
	for i := range j.Links {
		if errw := j.Links[i].NetworkLinks.Write(j.Links[i].Id.NetworkLinksPathFor(), false); errw != nil {
			resp.Failure(ErrLinkContainersFailed)
			return
		}
	}

	resp.Success(jobs.ResponseOk)
}
