package linux

import (
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
)

type purgeContainers struct {
	*cjobs.PurgeContainersRequest
}

func (p *purgeContainers) Execute(res jobs.Response) {
	Clean()
	res.Success(jobs.ResponseOk)
}
