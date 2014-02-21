package jobs

import (
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/systemd"
	"log"
	"os"
	"time"
)

type StartedContainerStateJobRequest struct {
	JobResponse
	JobRequest
	GearId gears.Identifier
	UserId string
}

func (j *StartedContainerStateJobRequest) Execute() {
	status, err := systemd.StartAndEnableUnit(systemd.SystemdConnection(), j.GearId.UnitNameFor(), j.GearId.UnitPathFor(), "fail")

	switch {
	case systemd.IsNoSuchUnit(err):
		j.Failure(ErrGearNotFound)
		return
	case err != nil:
		log.Printf("job_alter_container_state: Gear did not start: %+v", err)
		j.Failure(ErrGearStartFailed)
		return
	case status != "done":
		log.Printf("job_alter_container_state: Unit did not return 'done': %v", err)
		j.Failure(ErrGearStartFailed)
		return
	}

	w := j.SuccessWithWrite(JobResponseAccepted, true)
	fmt.Fprintf(w, "Gear %s starting\n", j.GearId)
}

type StoppedContainerStateJobRequest struct {
	JobResponse
	JobRequest
	GearId gears.Identifier
	UserId string
}

func (j *StoppedContainerStateJobRequest) Execute() {
	w := j.SuccessWithWrite(JobResponseAccepted, true)

	unitName := j.GearId.UnitNameFor()
	done := make(chan time.Time)

	ioerr := make(chan error)
	go func() {
		ioerr <- gears.WriteLogsTo(w, unitName, done)
	}()

	joberr := make(chan error)
	go func() {
		status, err := systemd.SystemdConnection().StopUnit(unitName, "fail")
		if err == nil && status != "done" {
			err = errors.New(fmt.Sprintf("Job status 'done' != %s", status))
		}
		joberr <- err
	}()

	var err error
	select {
	case err = <-ioerr:
		log.Printf("job_alter_container_state: Client hung up")
	case err = <-joberr:
		log.Printf("job_alter_container_state: Stop job done")
	case <-time.After(15 * time.Second):
		log.Printf("job_alter_container_state: Timeout waiting for stop completion")
	}
	close(done)

	switch {
	case systemd.IsNoSuchUnit(err):
		if _, err := os.Stat(j.GearId.UnitPathFor()); err == nil {
			fmt.Fprintf(w, "Gear %s is stopped\n", j.GearId)
		} else {
			fmt.Fprintf(w, "No such gear %s\n", j.GearId)
		}
	case err != nil:
		fmt.Fprintf(w, "Could not start gear: %s\n", err.Error())
	default:
		fmt.Fprintf(w, "Gear %s is stopped\n", j.GearId)
	}
}
