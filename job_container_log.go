package geard

import (
	"log"
	"os"
	"time"
)

type containerLogJobRequest struct {
	JobResponse
	jobRequest
	GearId Identifier
	UserId string
}

func (j *containerLogJobRequest) Execute() {
	if _, err := os.Stat(j.GearId.UnitPathFor()); err != nil {
		j.Failure(ErrGearNotFound)
		return
	}

	w := j.SuccessWithWrite(JobResponseOk, true)
	err := WriteLogsTo(w, j.GearId.UnitNameFor(), time.After(30*time.Second))
	if err != nil {
		log.Printf("job_container_log: Unable to fetch journal logs: %s\n", err.Error())
	}
}
