package linux

import (
	"log"
	"os"

	. "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/systemd"
)

type containerStatus struct {
	*ContainerStatusRequest
}

func (j *containerStatus) Execute(resp jobs.Response) {
	if _, err := os.Stat(j.Id.UnitPathFor()); err != nil {
		//log.Printf("container_status: Can't stat unit: %v", err)
		resp.Failure(ErrContainerNotFound)
		return
	}

	w := resp.SuccessWithWrite(jobs.ResponseOk, true, false)
	err := systemd.WriteStatusTo(w, j.Id.UnitNameFor())
	if err != nil {
		log.Printf("container_status: Unable to fetch container status logs: %s\n", err.Error())
	}
}
