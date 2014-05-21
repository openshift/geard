package jobs

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
)

func (c UnitResponses) Less(a, b int) bool {
	return c[a].Id < c[b].Id
}
func (c UnitResponses) Len() int {
	return len(c)
}
func (c UnitResponses) Swap(a, b int) {
	c[a], c[b] = c[b], c[a]
}

func (c ContainerUnitResponses) Less(a, b int) bool {
	return c[a].Id < c[b].Id
}
func (c ContainerUnitResponses) Len() int {
	return len(c)
}
func (c ContainerUnitResponses) Swap(a, b int) {
	c[a], c[b] = c[b], c[a]
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

type ListServerContainersResponse struct {
	ListContainersResponse
}

func (l *ListServerContainersResponse) WriteTableTo(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 8, 4, 1, ' ', tabwriter.DiscardEmptyColumns)
	if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", "ID", "SERVER", "ACTIVE", "SUB", "LOAD", "TYPE"); err != nil {
		return err
	}
	for i := range l.Containers {
		container := &l.Containers[i]
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", container.Id, container.Server, container.ActiveState, container.SubState, container.LoadState, container.JobType); err != nil {
			return err
		}
	}
	tw.Flush()
	return nil
}
