package jobs

import (
	"github.com/smarterclayton/geard/systemd"
	"github.com/smarterclayton/go-systemd/dbus"
	"log"
	"regexp"
	"sort"
)

type unitResponse struct {
	Id          string `json:"id"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
}
type units []unitResponse

func (c units) Less(a, b int) bool {
	return c[a].Id < c[b].Id
}
func (c units) Len() int {
	return len(c)
}
func (c units) Swap(a, b int) {
	c[a], c[b] = c[b], c[a]
}

func unitsMatching(re *regexp.Regexp, found func(name string, unit *dbus.UnitStatus)) error {
	all, err := systemd.Connection().ListUnits()
	if err != nil {
		return err
	}

	for _, unit := range all {
		if matched := re.MatchString(unit.Name); matched {
			name := re.FindStringSubmatch(unit.Name)[1]
			found(name, &unit)
		}
	}
	return nil
}

type ListContainersRequest struct {
	JobResponse
	JobRequest
}
type containerResponse struct {
	unitResponse
	LoadState string `json:"load_state"`
	JobType   string `json:"job_type,omitempty"`
}
type containers []containerResponse

func (c containers) Less(a, b int) bool {
	return c[a].Id < c[b].Id
}
func (c containers) Len() int {
	return len(c)
}
func (c containers) Swap(a, b int) {
	c[a], c[b] = c[b], c[a]
}

type listContainers struct {
	Containers containers `json:"containers"`
}

var reGearUnits = regexp.MustCompile("\\Agear-([^\\.]+)\\.service\\z")

func (j *ListContainersRequest) Execute() {
	r := listContainers{make(containers, 0)}

	if err := unitsMatching(reGearUnits, func(name string, unit *dbus.UnitStatus) {
		r.Containers = append(r.Containers, containerResponse{
			unitResponse{
				name,
				unit.ActiveState,
				unit.SubState,
			},
			unit.LoadState,
			unit.JobType,
		})
	}); err != nil {
		log.Printf("list_units: Unable to list units from systemd: %v", err)
		j.Failure(ErrListContainersFailed)
		return
	}

	sort.Sort(r.Containers)
	j.SuccessWithData(JobResponseOk, r)
}

type ListBuildsRequest struct {
	JobResponse
	JobRequest
}
type listBuilds struct {
	Builds units `json:"builds"`
}

var reBuildUnits = regexp.MustCompile("\\Abuild-([^\\.]+)\\.service\\z")

func (j *ListBuildsRequest) Execute() {
	r := listBuilds{make(units, 0)}

	if err := unitsMatching(reBuildUnits, func(name string, unit *dbus.UnitStatus) {
		r.Builds = append(r.Builds, unitResponse{
			name,
			unit.ActiveState,
			unit.SubState,
		})
	}); err != nil {
		log.Printf("list_units: Unable to list units from systemd: %v", err)
		j.Failure(ErrListContainersFailed)
		return
	}
	sort.Sort(r.Builds)
	j.SuccessWithData(JobResponseOk, r)
}
