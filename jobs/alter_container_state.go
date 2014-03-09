package jobs

import (
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/systemd"
	"log"
	"os"
	"time"
)

var rateLimitChanges uint64 = 400 * 1000 /* in microseconds */

func propIncludesString(value interface{}, key string) bool {
	if arr, ok := value.([]string); ok {
		for i := range arr {
			if arr[i] == key {
				return true
			}
		}
	}
	return false
}

func inStateOrTooSoon(unit string, active, transition bool, rateLimit uint64) (inState bool, tooSoon bool, markedActive bool) {
	if props, erru := systemd.Connection().GetUnitProperties(unit); erru == nil {

		markedActive = propIncludesString(props["WantedBy"], "container-active.target")

		switch props["ActiveState"] {
		case "active":
			if active {
				inState = true
				return
			}
		case "activating":
			if active {
				inState = true
				return
			} else if transition {
				tooSoon = true
				return
			}
		case "inactive", "failed":
			if !active {
				inState = true
				return
			}
		case "deactivating":
			if !active {
				inState = true
				return
			} else if transition {
				tooSoon = true
				return
			}
		}
		if arr, ok := props["Job"].([]interface{}); ok {
			if i, ok := arr[0].(int); ok {
				if i != 0 {
					log.Printf("alter_container_state: There is an enqueued job against unit %s: %d", unit, i)
					inState = true
					return
				}
			}
		}
		now := time.Now().UnixNano() / 1000
		if act, ok := props["ActiveEnterTimestamp"]; ok {
			if inact, ok := props["InactiveEnterTimestamp"]; ok {
				t1 := act.(uint64)
				t2 := inact.(uint64)
				if transition {
					// compare against the most recent value
					if t1 > t2 {
						t1, t2 = t2, t1
					}
				} else if !active {
					t1, t2 = t2, t1
				}
				if t2 > t1 {
					diff := uint64(now) - t2
					if diff < rateLimit {
						tooSoon = true
						return
					}
				}
			}
		}
	}
	return
}

type StartedContainerStateRequest struct {
	Id containers.Identifier
}

func (j *StartedContainerStateRequest) Execute(resp JobResponse) {
	unitName := j.Id.UnitNameFor()
	unitPath := j.Id.UnitPathFor()

	inState, tooSoon, markedActive := inStateOrTooSoon(unitName, true, false, rateLimitChanges)
	if inState {
		w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
		fmt.Fprintf(w, "Container %s starting\n", j.Id)
		return
	}
	if tooSoon {
		resp.Failure(ErrStartRequestThrottled)
		return
	}

	if !markedActive {
		if errs := containers.WriteContainerState(j.Id, true); errs != nil {
			log.Print("alter_container_state: Unable to write state file: ", errs)
			resp.Failure(ErrContainerStartFailed)
			return
		}
	}

	if err := systemd.EnableAndReloadUnit(systemd.Connection(), unitName, unitPath); err != nil {
		if systemd.IsNoSuchUnit(err) || systemd.IsFileNotFound(err) {
			resp.Failure(ErrContainerNotFound)
			return
		}
		log.Printf("alter_container_state: Could not enable container %s: %v", unitName, err)
		resp.Failure(ErrContainerStartFailed)
		return
	}

	if err := systemd.Connection().StartUnitJob(unitName, "fail"); err != nil {
		log.Printf("alter_container_state: Could not start container %s: %v", unitName, err)
		resp.Failure(ErrContainerStartFailed)
		return
	}

	w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
	fmt.Fprintf(w, "Container %s starting\n", j.Id)
}

type StoppedContainerStateRequest struct {
	Id containers.Identifier
}

func (j *StoppedContainerStateRequest) Execute(resp JobResponse) {
	inState, tooSoon, markedActive := inStateOrTooSoon(j.Id.UnitNameFor(), false, false, rateLimitChanges)
	if inState {
		w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
		fmt.Fprintf(w, "Container %s is stopped\n", j.Id)
		return
	}
	if tooSoon {
		resp.Failure(ErrStopRequestThrottled)
		return
	}

	if markedActive {
		if errs := containers.WriteContainerState(j.Id, false); errs != nil {
			log.Print("alter_container_state: Unable to write state file: ", errs)
			resp.Failure(ErrContainerStopFailed)
			return
		}
	}

	w := resp.SuccessWithWrite(JobResponseAccepted, true, false)

	unitName := j.Id.UnitNameFor()
	done := make(chan time.Time)

	ioerr := make(chan error)
	go func() {
		ioerr <- systemd.WriteLogsTo(w, unitName, 0, done)
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
			fmt.Fprintf(w, "Container %s is stopped\n", j.Id)
		} else {
			fmt.Fprintf(w, "No such container %s\n", j.Id)
		}
	case err != nil:
		fmt.Fprintf(w, "Could not start container: %s\n", err.Error())
	default:
		fmt.Fprintf(w, "Container %s is stopped\n", j.Id)
	}
}

type RestartContainerRequest struct {
	Id containers.Identifier
}

func (j *RestartContainerRequest) Execute(resp JobResponse) {
	unitName := j.Id.UnitNameFor()
	unitPath := j.Id.UnitPathFor()

	inState, tooSoon, markedActive := inStateOrTooSoon(unitName, false, true, rateLimitChanges)
	if inState {
		w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
		fmt.Fprintf(w, "Container %s restarting\n", j.Id)
		return
	}
	if tooSoon {
		resp.Failure(ErrRestartRequestThrottled)
		return
	}

	if !markedActive {
		if errs := containers.WriteContainerState(j.Id, true); errs != nil {
			log.Print("alter_container_state: Unable to write state file: ", errs)
			resp.Failure(ErrContainerRestartFailed)
			return
		}
	}

	if err := systemd.EnableAndReloadUnit(systemd.Connection(), unitName, unitPath); err != nil {
		if systemd.IsNoSuchUnit(err) || systemd.IsFileNotFound(err) {
			resp.Failure(ErrContainerNotFound)
			return
		}
		log.Printf("alter_container_state: Could not enable container %s: %v", unitName, err)
		resp.Failure(ErrContainerRestartFailed)
		return
	}

	if err := systemd.Connection().RestartUnitJob(unitName, "fail"); err != nil {
		log.Printf("alter_container_state: Could not restart container %s: %v", unitName, err)
		resp.Failure(ErrContainerRestartFailed)
		return
	}

	w := resp.SuccessWithWrite(JobResponseAccepted, true, false)
	fmt.Fprintf(w, "Container %s restarting\n", j.Id)
}
