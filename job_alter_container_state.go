package geard

import (
	"fmt"
	"io"
	"os"
	"time"
)

type startedContainerStateJobRequest struct {
	jobRequest
	GearId Identifier
	UserId string
	Output io.Writer
}

func (j *startedContainerStateJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Ensuring gear %s is started ... \n", j.GearId)

	status, err := StartAndEnableUnit(j.GearId.UnitNameFor(), j.GearId.UnitPathFor())

	switch {
	case IsNoSuchUnit(err):
		fmt.Fprintf(j.Output, "No such gear %s\n", j.GearId)
	case err != nil:
		fmt.Fprintf(j.Output, "Could not start gear %+v\n", err)
	case status != "done":
		fmt.Fprintf(j.Output, "Gear did not start successfully: %s\n", status)
	default:
		stdout, err := ProcessLogsFor(j.GearId)
		if err != nil {
			stdout = emptyReader
			fmt.Fprintf(j.Output, "Unable to fetch journal logs: %s\n", err.Error())
		}
		defer stdout.Close()
		go io.Copy(j.Output, stdout)

		time.Sleep(3 * time.Second)
		stdout.Close()

		fmt.Fprintf(j.Output, "Gear %s is started\n", j.GearId)
	}
}

func StartAndEnableUnit(name string, path string) (string, error) {
	status, err := SystemdConnection().StartUnit(name, "fail")
	switch {
	case IsNoSuchUnit(err), IsLoadFailed(err):
		if _, err := os.Stat(path); err == nil {
			_, _, err := SystemdConnection().EnableUnitFiles([]string{path}, false, false)
			if err == nil {
				return SystemdConnection().StartUnit(name, "fail")
			} else {
				return "", err
			}
		} else {
			return "", ErrNoSuchUnit
		}
	}
	return status, err
}

type stoppedContainerStateJobRequest struct {
	jobRequest
	GearId Identifier
	UserId string
	Output io.Writer
}

func (j *stoppedContainerStateJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Ensuring gear %s is stopped ... \n", j.GearId)

	// stop is a blocking operation
	stdout, err := ProcessLogsFor(j.GearId)
	if err != nil {
		stdout = emptyReader
		//fmt.Fprintf(j.Output, "Unable to fetch journal logs: %s\n", err.Error())
	}
	defer stdout.Close()
	go io.Copy(j.Output, stdout)

	unitName := j.GearId.UnitNameFor()
	status, err := SystemdConnection().StopUnit(unitName, "fail")
	stdout.Close()
	switch {
	case IsNoSuchUnit(err):
		if _, err := os.Stat(j.GearId.UnitPathFor()); err == nil {
			fmt.Fprintf(j.Output, "Gear %s is stopped\n", j.GearId)
		} else {
			fmt.Fprintf(j.Output, "No such gear %s\n", j.GearId)
		}
	case err != nil:
		fmt.Fprintf(j.Output, "Could not start gear: %s\n", err.Error())
	case status != "done":
		fmt.Fprintf(j.Output, "Gear did not start successfully: %s\n", status)
	default:
		fmt.Fprintf(j.Output, "Gear %s is stopped\n", j.GearId)
	}
}
