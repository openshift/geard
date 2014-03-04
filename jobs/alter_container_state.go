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

type StartedContainerStateRequest struct {
	Id gears.Identifier
}

func (j *StartedContainerStateRequest) Execute(resp JobResponse) {
	unitName := j.Id.UnitNameFor()
	unitPath := j.Id.UnitPathFor()

	in_state, too_soon := inStateOrTooSoon(unitName, true, rateLimitChanges)
	if in_state {
		w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
		fmt.Fprintf(w, "Gear %s starting\n", j.Id)
		return
	}
	if too_soon {
		resp.Failure(ErrStartRequestThrottled)
		return
	}

	if errs := gears.WriteGearState(j.Id, true); errs != nil {
		log.Print("alter_container_state: Unable to write state file: ", errs)
		resp.Failure(ErrGearStartFailed)
		return
	}

	if err := systemd.EnableAndReloadUnit(systemd.Connection(), unitName, unitPath); err != nil {
		if systemd.IsNoSuchUnit(err) || systemd.IsFileNotFound(err) {
			resp.Failure(ErrGearNotFound)
			return
		}
		log.Printf("alter_container_state: Could not enable gear %s: %v", unitName, err)
		resp.Failure(ErrGearStartFailed)
		return
	}

	if err := systemd.Connection().StartUnitJob(unitName, "fail"); err != nil {
		log.Printf("install_container: Could not start gear %s: %v", unitName, err)
		resp.Failure(ErrGearStartFailed)
		return
	}

	w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
	fmt.Fprintf(w, "Gear %s starting\n", j.Id)
}

type StoppedContainerStateRequest struct {
	Id gears.Identifier
}

func (j *StoppedContainerStateRequest) Execute(resp JobResponse) {
	in_state, too_soon := inStateOrTooSoon(j.Id.UnitNameFor(), false, rateLimitChanges)
	if in_state {
		w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
		fmt.Fprintf(w, "Gear %s is stopped\n", j.Id)
		return
	}
	if too_soon {
		resp.Failure(ErrStopRequestThrottled)
		return
	}

	if errs := gears.WriteGearState(j.Id, false); errs != nil {
		log.Print("alter_container_state: Unable to write state file: ", errs)
		resp.Failure(ErrGearStopFailed)
		return
	}

	w := resp.SuccessWithWrite(JobResponseAccepted, true, false)

	unitName := j.Id.UnitNameFor()
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
		log.Printf("alter_container_state: Client hung up")
		close(ioerr)
	case err = <-joberr:
		log.Printf("alter_container_state: Stop job done")
	case <-time.After(15 * time.Second):
		log.Printf("alter_container_state: Timeout waiting for stop completion")
	}
	close(done)

	select {
	case <-ioerr:
	}

	switch {
	case systemd.IsNoSuchUnit(err):
		if _, err := os.Stat(j.Id.UnitPathFor()); err == nil {
			fmt.Fprintf(w, "Gear %s is stopped\n", j.Id)
		} else {
			fmt.Fprintf(w, "No such gear %s\n", j.Id)
		}
	case err != nil:
		fmt.Fprintf(w, "Could not start gear: %s\n", err.Error())
	default:
		fmt.Fprintf(w, "Gear %s is stopped\n", j.Id)
	}
}
