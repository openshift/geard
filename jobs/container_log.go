package jobs

import (
	"github.com/smarterclayton/geard/gears"
	"log"
	"os"
	"time"
)

type ContainerLogRequest struct {
	GearId gears.Identifier
	UserId string
}

func (j *ContainerLogRequest) Execute(resp JobResponse) {
	if _, err := os.Stat(j.GearId.UnitPathFor()); err != nil {
		resp.Failure(ErrGearNotFound)
		return
	}

	w := resp.SuccessWithWrite(JobResponseOk, true, false)
	err := gears.WriteLogsTo(w, j.GearId.UnitNameFor(), 30, time.After(30*time.Second))
	if err != nil {
		log.Printf("job_container_log: Unable to fetch journal logs: %s\n", err.Error())
	}
}
