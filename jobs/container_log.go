package jobs

import (
	"github.com/smarterclayton/geard/gears"
	"log"
	"os"
	"time"
)

type ContainerLogJobRequest struct {
	JobResponse
	JobRequest
	GearId gears.Identifier
	UserId string
}

func (j *ContainerLogJobRequest) Execute() {
	if _, err := os.Stat(j.GearId.UnitPathFor()); err != nil {
		j.Failure(ErrGearNotFound)
		return
	}

	w := j.SuccessWithWrite(JobResponseOk, true)
	err := gears.WriteLogsTo(w, j.GearId.UnitNameFor(), time.After(30*time.Second))
	if err != nil {
		log.Printf("job_container_log: Unable to fetch journal logs: %s\n", err.Error())
	}
}
