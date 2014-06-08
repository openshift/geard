package linux

import (
	"log"
	"os"
	"time"

	. "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/systemd"
)

type containerLog struct {
	*ContainerLogRequest
}

func (j *containerLog) Execute(resp jobs.Response) {
	if _, err := os.Stat(j.Id.UnitPathFor()); err != nil {
		resp.Failure(ErrContainerNotFound)
		return
	}

	w := resp.SuccessWithWrite(jobs.ResponseOk, true, false)
	err := systemd.WriteLogsTo(w, j.Id.UnitNameFor(), 30, time.After(30*time.Second))
	if err != nil {
		log.Printf("job_container_log: Unable to fetch journal logs: %s\n", err.Error())
	}
}
