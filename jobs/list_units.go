package jobs

import (
	"fmt"
	"github.com/smarterclayton/geard/systemd"
	"github.com/smarterclayton/go-systemd/dbus"
	"io"
	"log"
	"regexp"
	"sort"
	"text/tabwriter"
)

type unitResponse struct {
	Id          string
	ActiveState string
	SubState    string
}
type unitResponses []unitResponse

func (c unitResponses) Less(a, b int) bool {
	return c[a].Id < c[b].Id
}
func (c unitResponses) Len() int {
	return len(c)
}
func (c unitResponses) Swap(a, b int) {
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
	LoadState string
	JobType   string `json:"JobType,omitempty"`
}
type containerResponses []containerResponse

func (c containerResponses) Less(a, b int) bool {
	return c[a].Id < c[b].Id
}
func (c containerResponses) Len() int {
	return len(c)
}
func (c containerResponses) Swap(a, b int) {
	c[a], c[b] = c[b], c[a]
}

type listContainers struct {
	Containers containerResponses
}

func (l *listContainers) WriteTableTo(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 8, 4, 1, ' ', tabwriter.DiscardEmptyColumns)
	if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", "ID", "ACTIVE", "SUB", "LOAD", "TYPE"); err != nil {
		return err
	}
	for i := range l.Containers {
		container := &l.Containers[i]
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", container.Id, container.ActiveState, container.SubState, container.LoadState, container.JobType); err != nil {
			return err
		}
	}
	tw.Flush()
	return nil
}

var reContainerUnits = regexp.MustCompile("\\Acontainer-([^\\.]+)\\.service\\z")

func (j *ListContainersRequest) Execute(resp JobResponse) {
	r := &listContainers{make(containerResponses, 0)}

	if err := unitsMatching(reContainerUnits, func(name string, unit *dbus.UnitStatus) {
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
	Builds unitResponses
}

var reBuildUnits = regexp.MustCompile("\\Abuild-([^\\.]+)\\.service\\z")

func (j *ListBuildsRequest) Execute(resp JobResponse) {
	r := listBuilds{make(unitResponses, 0)}

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
