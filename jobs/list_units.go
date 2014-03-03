package jobs

import (
	"github.com/smarterclayton/geard/systemd"
	"github.com/smarterclayton/go-systemd/dbus"
	"log"
	"regexp"
	"sort"
)

type unitResponse struct {
	Id          string
	ActiveState string
	SubState    string
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

func (j *ListContainersRequest) Execute(resp JobResponse) {
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
		resp.Failure(ErrListContainersFailed)
		return
	}

	sort.Sort(r.Containers)
	resp.SuccessWithData(JobResponseOk, r)
}

type ListBuildsRequest struct {
}
type listBuilds struct {
	Builds units `json:"builds"`
}

var reBuildUnits = regexp.MustCompile("\\Abuild-([^\\.]+)\\.service\\z")

func (j *ListBuildsRequest) Execute(resp JobResponse) {
	r := listBuilds{make(units, 0)}

	if err := unitsMatching(reBuildUnits, func(name string, unit *dbus.UnitStatus) {
		r.Builds = append(r.Builds, unitResponse{
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
	resp.SuccessWithData(JobResponseOk, r)
}
