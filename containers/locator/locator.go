package locator

import (
	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/transport"
)

// A container resource
const ResourceTypeContainer cmd.ResourceType = "ctr"

func NewContainerLocators(t transport.Transport, values ...string) (cmd.Locators, error) {
	locators, err := cmd.NewResourceLocators(t, ResourceTypeContainer, values...)
	if err != nil {
		return cmd.Locators{}, err
	}
	for i := range locators {
		_, err := containers.NewIdentifier(locators[i].(*cmd.ResourceLocator).Id)
		if err != nil {
			return cmd.Locators{}, err
		}
	}
	return locators, nil
}

func AsIdentifier(locator cmd.Locator) containers.Identifier {
	id, _ := containers.NewIdentifier(locator.(*cmd.ResourceLocator).Id)
	return id
}
