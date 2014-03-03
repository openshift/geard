package jobs

import (
	"github.com/smarterclayton/geard/gears"
	"log"
)

type ContainerPortsJobRequest struct {
	GearId gears.Identifier
	UserId string
}

func (j *ContainerPortsJobRequest) Execute(resp JobResponse) {
	portPairs, err := gears.GetExistingPorts(j.GearId)
	if err != nil {
		log.Printf("job_container_ports_log: Unable to find unit: %s\n", err.Error())
		resp.Failure(ErrGearNotFound)
		return
	}

	resp.SuccessWithData(JobResponseAccepted, portPairs)
}
