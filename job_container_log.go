package geard

import (
	"fmt"
	"io"
	"os"
	"time"
)

type containerLogJobRequest struct {
	jobRequest
	GearId Identifier
	UserId string
	Output io.Writer
}

func (j *containerLogJobRequest) Execute() {
	if _, err := os.Stat(j.GearId.UnitPathFor()); err != nil {
		return
	}

	err := WriteLogsTo(j.Output, j.GearId.UnitNameFor(), 30*time.Second)
	if err != nil {
		fmt.Fprintf(j.Output, "Unable to fetch journal logs: %s\n", err.Error())
	}
}
