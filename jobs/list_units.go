package jobs

import (
	"fmt"
	"github.com/smarterclayton/geard/containers"
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
type ContainerUnitResponse struct {
	unitResponse
	LoadState string
	JobType   string `json:"JobType,omitempty"`
	// Used by consumers
	Server string `json:"Server,omitempty"`
}
type ContainerUnitResponses []ContainerUnitResponse

func (c ContainerUnitResponses) Less(a, b int) bool {
	return c[a].Id < c[b].Id
}
func (c ContainerUnitResponses) Len() int {
	return len(c)
}
func (c ContainerUnitResponses) Swap(a, b int) {
	c[a], c[b] = c[b], c[a]
}

type ListContainersResponse struct {
	Containers ContainerUnitResponses
}

func (r *ListContainersResponse) Append(other *ListContainersResponse) {
	r.Containers = append(r.Containers, other.Containers...)
}
func (r *ListContainersResponse) Sort() {
	sort.Sort(r.Containers)
}

func (l *ListContainersResponse) WriteTableTo(w io.Writer) error {
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

var reContainerUnits = regexp.MustCompile("\\A" + regexp.QuoteMeta(containers.IdentifierPrefix) + "([^\\.]+)\\.service\\z")

func (j *ListContainersRequest) Execute(resp JobResponse) {
	r := &ListContainersResponse{make(ContainerUnitResponses, 0)}

	if err := unitsMatching(reContainerUnits, func(name string, unit *dbus.UnitStatus) {
		r.Containers = append(r.Containers, ContainerUnitResponse{
			unitResponse{
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
