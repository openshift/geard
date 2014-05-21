// +build linux

package jobs

import (
	"log"

	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
)

func (j *ContainerPortsRequest) Execute(resp jobs.Response) {
	portPairs, err := containers.GetExistingPorts(j.Id)
	if err != nil {
		log.Printf("job_container_ports_log: Unable to find unit: %s\n", err.Error())
		resp.Failure(ErrContainerNotFound)
		return
	}

	resp.SuccessWithData(jobs.ResponseAccepted, ContainerPortsResponse{portPairs})
}
