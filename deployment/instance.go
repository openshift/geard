package deployment

import (
	"errors"
	"fmt"

	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/transport"
)

// A container that has been created on a server
type Instance struct {
	// The id of this instance
	Id containers.Identifier
	// The image used by this instance
	Image string
	// The container definition this is from
	From string
	// The deployed location - nil for not deployed
	On *string `json:"On,omitempty"`
	// The mapping of internal, external, and remote ports
	Ports PortMappings `json:"Ports,omitempty"`

	Notify bool

	// Was this instance added.
	add bool
	// Is this instance flagged for removal
	remove bool

	// The container this instance is associated with
	container *Container
	// The resolved locator of an instance
	on transport.Locator
	// The generated links for this instance
	links InstanceLinks
	// A cached hostname for this instance
	hostname string
}
type Instances []Instance
type InstanceRefs []*Instance

func (i *Instance) NetworkLinks() containers.NetworkLinks {
	return i.links.NetworkLinks()
}

func (i *Instance) Added() bool {
	return i.add
}

func (i *Instance) MarkRemoved() {
	i.remove = true
}

func (i *Instance) Place(on transport.Locator) {
	i.on = on
	s := on.String()
	i.On = &s
}
func (i *Instance) ResolveHostname() (string, error) {
	if i.on == nil {
		return "", errors.New(fmt.Sprintf("No locator available for this instance (can't resolve from %s)", i.On))
	}
	return i.on.ResolveHostname()
}

func (i *Instance) EnvironmentVariables() {
}

func (instances Instances) Find(id containers.Identifier) (*Instance, bool) {
	for i := range instances {
		if string(instances[i].Id) == string(id) {
			return &instances[i], true
		}
	}
	return nil, false
}

func (instances Instances) References() InstanceRefs {
	refs := make(InstanceRefs, 0, 5)
	for i := range instances {
		refs = append(refs, &instances[i])
	}
	return refs
}

func (instances Instances) ReferencesFor(name string) InstanceRefs {
	refs := make(InstanceRefs, 0, 5)
	for i := range instances {
		if instances[i].From == name {
			refs = append(refs, &instances[i])
		}
	}
	return refs
}

func (refs Instances) Added() InstanceRefs {
	adds := make(InstanceRefs, 0, len(refs))
	for i := range refs {
		if refs[i].add {
			adds = append(adds, &refs[i])
		}
	}
	return adds
}

func (refs Instances) Linked() InstanceRefs {
	linked := make(InstanceRefs, 0, len(refs))
	for i := range refs {
		if len(refs[i].links) > 0 {
			linked = append(linked, &refs[i])
		}
	}
	return linked
}
