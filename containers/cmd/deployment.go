package cmd

import (
	"errors"
	"fmt"

	"github.com/openshift/geard/cmd"
	cloc "github.com/openshift/geard/containers/locator"
	"github.com/openshift/geard/deployment"
	"github.com/openshift/geard/transport"
)

// Return a set of container locators from the specified deployment
// descriptor.
func ExtractContainerLocatorsFromDeployment(t transport.Transport, path string, args *[]string) error {
	if path == "" {
		return nil
	}
	deployment, err := deployment.NewDeploymentFromFile(path)
	if err != nil {
		return err
	}
	locators, err := LocatorsForDeploymentInstances(t, deployment.Instances.References())
	if err != nil {
		return err
	}
	if len(locators) == 0 {
		return errors.New(fmt.Sprintf("There are no deployed instances listed in %s", path))
	}
	for i := range locators {
		*args = append(*args, locators[i].Identity())
	}
	return nil
}

func LocatorsForDeploymentInstances(t transport.Transport, instances deployment.InstanceRefs) (cmd.Locators, error) {
	locators := make(cmd.Locators, 0, len(instances))
	for _, instance := range instances {
		if instance.On != nil {
			locator, err := t.LocatorFor(*instance.On)
			if err != nil {
				return cmd.Locators{}, err
			}
			resource := &cmd.ResourceLocator{cloc.ResourceTypeContainer, string(instance.Id), locator}
			locators = append(locators, resource)
		}
	}
	return locators, nil
}
