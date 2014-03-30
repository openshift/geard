package deployment

import (
	"github.com/openshift/geard/cmd"
)

type PlacementStrategy interface {
	// Return true if the location of an existing container is no
	// longer valid.
	RemoveFromLocation(cmd.Locator) bool
	// Allow the strategy to determine which location will host a
	// container by setting Instance.On for each container in added.
	// Failing to set an "On" for a container will return an error.
	//
	// Placement strategies may optionally suggest containers to remove
	// when scaling down by invoking Instance.MarkRemoved(). The caller
	// will then use those suggestions when determining the containers
	// to purge.
	Assign(added InstanceRefs, containers Containers) error
}

type SimplePlacement cmd.Locators

func (p SimplePlacement) RemoveFromLocation(on cmd.Locator) bool {
	return !cmd.Locators(p).Has(on)
}
func (p SimplePlacement) Assign(added InstanceRefs, containers Containers) error {
	locators := cmd.Locators(p)
	pos := 0
	for i := range added {
		instance := added[i]
		if len(locators) > 0 {
			host, _ := cmd.NewHostLocator(locators[pos%len(locators)].HostIdentity())
			instance.On = host
			pos++
		} else {
			instance.MarkRemoved()
		}
	}
	return nil
}
