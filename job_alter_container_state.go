package geard

import (
	"fmt"
	"io"
	"time"
)

type startedContainerStateJobRequest struct {
	jobRequest
	GearId GearIdentifier
	UserId string
	Output io.Writer
}

func (j *startedContainerStateJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Ensuring gear %s is started ... \n", j.GearId)

	unitName := UnitNameForGear(j.GearId)
	status, err := SystemdConnection().StartUnit(unitName, "fail")

	switch {
	case ErrNoSuchUnit(err):
		fmt.Fprintf(j.Output, "No such gear %s\n", j.GearId)
	case err != nil:
		fmt.Fprintf(j.Output, "Could not start gear %s\n", err.Error())
	case status != "done":
		fmt.Fprintf(j.Output, "Gear did not start successfully: %s\n", status)
	default:
		stdout, err := ProcessLogsForGear(j.GearId)
		if err != nil {
			stdout = emptyReader
			fmt.Fprintf(j.Output, "Unable to fetch journal logs: %s\n", err.Error())
		}
		defer stdout.Close()
		go io.Copy(j.Output, stdout)

		time.Sleep(3 * time.Second)
		stdout.Close()

		fmt.Fprintf(j.Output, "\nGear %s is started\n", j.GearId)
	}
}

type stoppedContainerStateJobRequest struct {
	jobRequest
	GearId GearIdentifier
	UserId string
	Output io.Writer
}

func (j *stoppedContainerStateJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Ensuring gear %s is stopped ... \n", j.GearId)

	// stop is a blocking operation
	stdout, err := ProcessLogsForGear(j.GearId)
	if err != nil {
		stdout = emptyReader
		//fmt.Fprintf(j.Output, "Unable to fetch journal logs: %s\n", err.Error())
	}
	defer stdout.Close()
	go io.Copy(j.Output, stdout)

	unitName := UnitNameForGear(j.GearId)
	status, err := SystemdConnection().StopUnit(unitName, "fail")
	stdout.Close()
	switch {
	case ErrNoSuchUnit(err):
		fmt.Fprintf(j.Output, "No such gear %s\n", j.GearId)
	case err != nil:
		fmt.Fprintf(j.Output, "Could not start gear: %s\n", err.Error())
	case status != "done":
		fmt.Fprintf(j.Output, "Gear did not start successfully: %s\n", status)
	default:
		fmt.Fprintf(j.Output, "\nGear %s is stopped\n", j.GearId)
	}
}
