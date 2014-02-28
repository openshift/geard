package jobs

import (
	"github.com/smarterclayton/geard/gears"
	"log"
	"os"
	"time"
)

type ContainerLogRequest struct {
	JobResponse
	JobRequest
	GearId gears.Identifier
	UserId string
}

func (j *ContainerLogRequest) Execute() {
	if _, err := os.Stat(j.GearId.UnitPathFor()); err != nil {
		j.Failure(ErrGearNotFound)
		return
	}

	w := j.SuccessWithWrite(JobResponseOk, true)
	err := gears.WriteLogsTo(w, j.GearId.UnitNameFor(), 30, time.After(30*time.Second))
	if err != nil {
		log.Printf("job_container_log: Unable to fetch journal logs: %s\n", err.Error())
	}
}
