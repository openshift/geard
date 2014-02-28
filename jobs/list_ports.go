package jobs

import (
	"github.com/smarterclayton/geard/gears"
	"log"
)

type ContainerPortsJobRequest struct {
	JobResponse
	JobRequest
	GearId gears.Identifier
	UserId string
}

func (j *ContainerPortsJobRequest) Execute() {
	portPairs, err := gears.GetExistingPorts(j.GearId)
	if err != nil {
		log.Printf("job_container_ports_log: Unable to find unit: %s\n", err.Error())
		j.Failure(ErrGearNotFound)
		return
	}

	j.SuccessWithData(JobResponseAccepted, portPairs)
}
