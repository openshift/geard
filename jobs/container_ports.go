package jobs

import (
	"github.com/smarterclayton/geard/gears"
	"log"
)

type ContainerPortsRequest struct {
	GearId gears.Identifier
	UserId string
}

type containerPortsResponse struct {
	Ports gears.PortPairs
}

func (j *ContainerPortsRequest) Execute(resp JobResponse) {
	portPairs, err := gears.GetExistingPorts(j.GearId)
	if err != nil {
		log.Printf("job_container_ports_log: Unable to find unit: %s\n", err.Error())
		resp.Failure(ErrGearNotFound)
		return
	}

	resp.SuccessWithData(JobResponseAccepted, containerPortsResponse{portPairs})
}
