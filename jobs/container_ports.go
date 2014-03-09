package jobs

import (
	"github.com/smarterclayton/geard/containers"
	"log"
)

type ContainerPortsRequest struct {
	Id containers.Identifier
}

type containerPortsResponse struct {
	Ports containers.PortPairs
}

func (j *ContainerPortsRequest) Execute(resp JobResponse) {
	portPairs, err := containers.GetExistingPorts(j.Id)
	if err != nil {
		log.Printf("job_container_ports_log: Unable to find unit: %s\n", err.Error())
		resp.Failure(ErrContainerNotFound)
		return
	}

	resp.SuccessWithData(JobResponseAccepted, containerPortsResponse{portPairs})
}
