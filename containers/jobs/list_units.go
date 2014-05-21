// +build linux

package jobs

import (
	"log"
	"regexp"
	"sort"

	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/go-systemd/dbus"
)

func unitsMatching(re *regexp.Regexp, found func(name string, unit *dbus.UnitStatus)) error {
	all, err := systemd.Connection().ListUnits()
	if err != nil {
		return err
	}

	for i := range all {
		unit := &all[i]
		if matched := re.MatchString(unit.Name); matched {
			name := re.FindStringSubmatch(unit.Name)[1]
			found(name, unit)
		}
	}
	return nil
}

var reContainerUnits = regexp.MustCompile("\\A" + regexp.QuoteMeta(containers.IdentifierPrefix) + "([^\\.]+)\\.service\\z")

func (j *ListContainersRequest) Execute(resp jobs.Response) {
	r := &ListContainersResponse{make(ContainerUnitResponses, 0)}

	if err := unitsMatching(reContainerUnits, func(name string, unit *dbus.UnitStatus) {
		if unit.LoadState == "not-found" || unit.LoadState == "masked" {
			return
		}
		r.Containers = append(r.Containers, ContainerUnitResponse{
			UnitResponse{
				name,
				unit.ActiveState,
				unit.SubState,
			},
			unit.LoadState,
			unit.JobType,
			"",
		})
	}); err != nil {
		log.Printf("list_units: Unable to list units from systemd: %v", err)
		resp.Failure(ErrListContainersFailed)
		return
	}

	r.Sort()
	resp.SuccessWithData(jobs.ResponseOk, r)
}

var reBuildUnits = regexp.MustCompile("\\Abuild-([^\\.]+)\\.service\\z")

func (j *ListBuildsRequest) Execute(resp jobs.Response) {
	r := ListBuildsResponse{make(UnitResponses, 0)}

	if err := unitsMatching(reBuildUnits, func(name string, unit *dbus.UnitStatus) {
		r.Builds = append(r.Builds, UnitResponse{
			name,
			unit.ActiveState,
			unit.SubState,
		})
	}); err != nil {
		log.Printf("list_units: Unable to list units from systemd: %v", err)
		resp.Failure(ErrListContainersFailed)
		return
	}
	sort.Sort(r.Builds)
	resp.SuccessWithData(jobs.ResponseOk, r)
}
