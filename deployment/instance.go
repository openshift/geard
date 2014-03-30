package deployment

import (
	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
)

// A container that has been created on a server
type Instance struct {
	// The id of this instance
	Id containers.Identifier
	// The image used by this instance
	Image string
	// The container definition this is from
	From string
	// The host system this is or should be deployed on
	On *cmd.HostLocator `json:"On,omitempty"`
	// The mapping of internal, external, and remote ports
	Ports PortMappings `json:"Ports,omitempty"`

	// Was this instance added.
	add bool
	// Is this instance flagged for removal
	remove bool

	// The container this instance is associated with
	container *Container
	// The generated links for this instance
	links InstanceLinks
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

func (i *Instance) ResolvedHostname() string {
	return i.On.ResolvedHostname()
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

func (instances Instances) ReferencesFor(name string) InstanceRefs {
	refs := make(InstanceRefs, 0, 5)
	for i := range instances {
		if instances[i].From == name {
			refs = append(refs, &instances[i])
		}
	}
	return refs
}

func (refs Instances) Ids() (ids []cmd.Locator) {
	ids = make([]cmd.Locator, 0, len(refs))
	for i := range refs {
		ids = append(ids, &cmd.ContainerLocator{*refs[i].On, refs[i].Id})
	}
	return
}

func (refs Instances) AddedIds() (ids []cmd.Locator) {
	ids = make([]cmd.Locator, 0, len(refs))
	for i := range refs {
		if refs[i].add {
			ids = append(ids, &cmd.ContainerLocator{*refs[i].On, refs[i].Id})
		}
	}
	return
}

func (refs Instances) LinkedIds() (ids []cmd.Locator) {
	ids = make([]cmd.Locator, 0, len(refs))
	for i := range refs {
		if len(refs[i].links) > 0 {
			ids = append(ids, &cmd.ContainerLocator{*refs[i].On, refs[i].Id})
		}
	}
	return
}

func (refs InstanceRefs) Ids() (ids []cmd.Locator) {
	ids = make([]cmd.Locator, 0, len(refs))
	for i := range refs {
		if refs[i] != nil {
			ids = append(ids, &cmd.ContainerLocator{*refs[i].On, refs[i].Id})
		}
	}
	return
}
