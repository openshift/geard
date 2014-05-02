package deployment

import (
	"github.com/openshift/geard/transport"
)

type PlacementStrategy interface {
	// Return true if the location of an existing container is no
	// longer valid.
	RemoveFromLocation(locator transport.Locator) bool
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

type SimplePlacement transport.Locators

func (p SimplePlacement) RemoveFromLocation(on transport.Locator) bool {
	for _, l := range transport.Locators(p) {
		if l.String() == on.String() {
			return false
		}
	}
	return true
}
func (p SimplePlacement) Assign(added InstanceRefs, containers Containers) error {
	locators := transport.Locators(p)
	pos := 0
	for i := range added {
		instance := added[i]
		if len(locators) > 0 {
			locator := locators[pos%len(locators)]
			instance.Place(locator)
			pos++
		} else {
			instance.MarkRemoved()
		}
	}
	return nil
}
