package linux

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	. "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/go-systemd/dbus"
)

func unitsMatching(conn systemd.Systemd, includeInactive bool, re *regexp.Regexp, found func(name string, unit *dbus.UnitStatus)) error {
	all, err := conn.ListUnits()
	if err != nil {
		return err
	}

	covered := make(map[string]int)
	if includeInactive {
		unitsPath := filepath.Join(config.ContainerBasePath(), "units")
		buckets, err := ioutil.ReadDir(unitsPath)
		if err != nil {
			return err
		}
		for i := range buckets {
			if buckets[i].IsDir() {
				dirPath := filepath.Join(unitsPath, buckets[i].Name())
				files, err := ioutil.ReadDir(dirPath)
				if err != nil {
					return err
				}
				for j := range files {
					if !files[j].IsDir() && re.MatchString(files[j].Name()) {
						covered[files[j].Name()] = 0
					}
				}
			}
		}
	}

	for i := range all {
		unit := &all[i]
		if matched := re.MatchString(unit.Name); matched {
			name := re.FindStringSubmatch(unit.Name)[1]
			covered[unit.Name] = 1
			found(name, unit)
		}
	}

	if includeInactive {
		for k := range covered {
			if covered[k] == 0 {
				name := re.FindStringSubmatch(k)[1]
				unit := &dbus.UnitStatus{Name: name, ActiveState: "inactive", LoadState: "loaded", SubState: "dead"}
				found(name, unit)
			}
		}
	}

	return nil
}

var reContainerUnits = regexp.MustCompile("\\A" + regexp.QuoteMeta(containers.IdentifierPrefix) + "(" + containers.IdentifierSuffixPattern + ")\\.service\\z")


type listContainers struct {
	*ListContainersRequest
	systemd systemd.Systemd
}

func (j *listContainers) Execute(resp jobs.Response) {
	r := &ListContainersResponse{make(ContainerUnitResponses, 0)}

	if err := unitsMatching(j.systemd, j.IncludeInactive, reContainerUnits, func(name string, unit *dbus.UnitStatus) {
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

type listBuilds struct {
	*ListBuildsRequest
	systemd systemd.Systemd
}

func (j *listBuilds) Execute(resp jobs.Response) {
	r := ListBuildsResponse{make(UnitResponses, 0)}

	if err := unitsMatching(j.systemd, false, reBuildUnits, func(name string, unit *dbus.UnitStatus) {
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
