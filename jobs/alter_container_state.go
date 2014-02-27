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

var rateLimitChanges uint64 = 400 * 1000 /* in microseconds */

func inStateOrTooSoon(unit string, active bool, rateLimit uint64) (bool, bool) {
	if props, erru := systemd.Connection().GetUnitProperties(unit); erru == nil {
		switch props["ActiveState"] {
		case "active", "activating":
			if active {
				return true, false
			}
		case "inactive", "deactivating", "failed":
			if !active {
				return true, false
			}
		}
		if arr, ok := props["Job"].([]interface{}); ok {
			if i, ok := arr[0].(int); ok {
				if i != 0 {
					log.Printf("alter_container_state: There is an enqueued job against unit %s: %d", unit, i)
					return true, false
				}
			}
		}
		now := time.Now().UnixNano() / 1000
		if act, ok := props["ActiveEnterTimestamp"]; ok {
			if inact, ok := props["InactiveEnterTimestamp"]; ok {
				t1 := act.(uint64)
				t2 := inact.(uint64)
				if !active {
					t1, t2 = t2, t1
				}
				if t2 > t1 {
					diff := uint64(now) - t2
					if diff < rateLimit {
						return false, true
					}
				}
			}
		}
	}
	return false, false
}

type StartedContainerStateJobRequest struct {
	JobResponse
	JobRequest
	GearId gears.Identifier
	UserId string
}

func (j *StartedContainerStateJobRequest) Execute() {
	unitName := j.GearId.UnitNameFor()
	unitPath := j.GearId.UnitPathFor()

	in_state, too_soon := inStateOrTooSoon(unitName, true, rateLimitChanges)
	if in_state {
		w := j.SuccessWithWrite(JobResponseAccepted, true)
		fmt.Fprintf(w, "Gear %s starting\n", j.GearId)
		return
	}
	if too_soon {
		j.Failure(ErrStartRequestThrottled)
		return
	}

	if errs := gears.WriteGearState(j.GearId, true); errs != nil {
		log.Print("job_alter_container_state: Unable to write state file: ", errs)
		j.Failure(ErrGearStartFailed)
		return
	}

	if err := systemd.EnableAndReloadUnit(systemd.Connection(), unitName, unitPath); err != nil {
		log.Printf("job_alter_container_state: Could not enable gear %s: %v", unitName, err)
		j.Failure(ErrGearStartFailed)
		return
	}

	if err := systemd.Connection().StartUnitJob(unitName, "fail"); err != nil {
		log.Printf("job_create_container: Could not start gear %s: %v", unitName, err)
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
	in_state, too_soon := inStateOrTooSoon(j.GearId.UnitNameFor(), false, rateLimitChanges)
	if in_state {
		w := j.SuccessWithWrite(JobResponseAccepted, true)
		fmt.Fprintf(w, "Gear %s is stopped\n", j.GearId)
		return
	}
	if too_soon {
		j.Failure(ErrStopRequestThrottled)
		return
	}

	if errs := gears.WriteGearState(j.GearId, false); errs != nil {
		log.Print("job_alter_container_state: Unable to write state file: ", errs)
		j.Failure(ErrGearStopFailed)
		return
	}

	w := j.SuccessWithWrite(JobResponseAccepted, true)

	unitName := j.GearId.UnitNameFor()
	done := make(chan time.Time)

	ioerr := make(chan error)
	go func() {
		ioerr <- gears.WriteLogsTo(w, unitName, 0, done)
	}()

	joberr := make(chan error)
	go func() {
		status, err := systemd.Connection().StopUnit(unitName, "fail")
		if err == nil && status != "done" {
			err = errors.New(fmt.Sprintf("Job status 'done' != %s", status))
		}
		joberr <- err
	}()

	var err error
	select {
	case err = <-ioerr:
		log.Printf("job_alter_container_state: Client hung up")
		close(ioerr)
	case err = <-joberr:
		log.Printf("job_alter_container_state: Stop job done")
	case <-time.After(15 * time.Second):
		log.Printf("job_alter_container_state: Timeout waiting for stop completion")
	}
	close(done)

	select {
	case <-ioerr:
	}

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
