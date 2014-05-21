// +build linux

package jobs

import (
	"github.com/openshift/geard/jobs"
)

func (p *PurgeContainersRequest) Execute(res jobs.Response) {
	Clean()
}
