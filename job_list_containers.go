package geard

import (
	"fmt"
	"github.com/smarterclayton/go-systemd/dbus"
	"log"
	"regexp"
)

type listContainersRequest struct {
	JobResponse
	jobRequest
}

func (j *listContainersRequest) Execute() {
	conn, errc := NewSystemdConnection()
	w := j.SuccessWithWrite(JobResponseAccepted, true)

	if errc != nil {
		log.Print("job_list_containers:", errc)
		fmt.Fprintf(w, "Unable to watch start status", errc)
		return
	}

	if err := conn.Subscribe(); err != nil {
		log.Print("job_list_containers:", err)
		fmt.Fprintf(w, "Unable to watch start status", errc)
		return
	}
	defer conn.Unsubscribe()

	units, err := conn.ListUnits()

	if err != nil {
		fmt.Fprint(w, "Couldn't list units")
		return
	}

	var gearUnits []dbus.UnitStatus
	re := regexp.MustCompile("gear-(.*)\\.service")

	for _, unit := range units {
		if matched := re.MatchString(unit.Name); matched {
			gearUnits = append(gearUnits, unit)
		}
	}

	for _, unit := range gearUnits {
		res := re.FindStringSubmatch(unit.Name)
		fmt.Fprintf(w, "%s\n", res[1])
	}
}
