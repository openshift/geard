package jobs

import (
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/port"
	"log"
)

type ContainerPortsRequest struct {
	Id containers.Identifier
}

type containerPortsResponse struct {
	Ports port.PortPairs
}

func (j *ContainerPortsRequest) Execute(resp jobs.JobResponse) {
	portPairs, err := containers.GetExistingPorts(j.Id)
	if err != nil {
		log.Printf("job_container_ports_log: Unable to find unit: %s\n", err.Error())
		resp.Failure(ErrContainerNotFound)
		return
	}

	resp.SuccessWithData(jobs.JobResponseAccepted, containerPortsResponse{portPairs})
}
