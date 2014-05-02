package jobs

import (
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/systemd"
	"log"
	"os"
)

type ContainerStatusRequest struct {
	Id containers.Identifier
}

func (j *ContainerStatusRequest) Execute(resp JobResponse) {
	if _, err := os.Stat(j.Id.UnitPathFor()); err != nil {
		//log.Printf("container_status: Can't stat unit: %v", err)
		resp.Failure(ErrContainerNotFound)
		return
	}

	w := resp.SuccessWithWrite(JobResponseOk, true, false)
	err := systemd.WriteStatusTo(w, j.Id.UnitNameFor())
	if err != nil {
		log.Printf("container_status: Unable to fetch container status logs: %s\n", err.Error())
	}
}
