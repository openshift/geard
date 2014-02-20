package jobs

import (
	"log"
	"os"
	"time"
	"github.com/smarterclayton/geard/gear"
)

type ContainerLogJobRequest struct {
	JobResponse
	JobRequest
	GearId gear.Identifier
	UserId string
}

func (j *ContainerLogJobRequest) Execute() {
	if _, err := os.Stat(j.GearId.UnitPathFor()); err != nil {
		j.Failure(ErrGearNotFound)
		return
	}

	w := j.SuccessWithWrite(JobResponseOk, true)
	err := gear.WriteLogsTo(w, j.GearId.UnitNameFor(), 30*time.Second)
	if err != nil {
		log.Printf("job_container_log: Unable to fetch journal logs: %s\n", err.Error())
	}
}
