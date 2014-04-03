// Classes that define relationships between containers (links)
// and simple strategies for placement.
package deployment

import (
	"encoding/json"
	"errors"
	"github.com/openshift/geard/containers"
	"log"
	"os"
	"strconv"
)

const DistributeAffinity = "distribute"

// A description of a deployment
type Deployment struct {
	Containers Containers
	Instances  Instances

	IdPrefix     string
	RandomizeIds bool
}

func NewDeploymentFromFile(path string) (*Deployment, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	deployment := &Deployment{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(deployment); err != nil {
		return nil, err
	}
	return deployment, nil
}

func (d Deployment) Describe(placement PlacementStrategy) (next *Deployment, removed InstanceRefs, err error) {
	// copy the container list and clear any intermediate state
	sources := d.Containers.Copy()

	// assign instances to containers or the remove list
	for _, instance := range d.Instances {
		// is the instance invalid or no longer part of the cluster
		if instance.On == nil {
			continue
		}
		if placement.RemoveFromLocation(instance.On) {
			removed = append(removed, &instance)
			continue
		}
		// locate the container
		c, found := sources.Find(instance.From)
		if !found {
			removed = append(removed, &instance)
			continue
		}
		c.AddInstance(&instance)
	}

	// create new instances for each container
	added := make(InstanceRefs, 0)
	for i := range sources {
		c := &sources[i]
		if errc := d.createInstances(c); errc != nil {
			err = errc
			return
		}
		for j := range c.instances {
			if c.instances[j].add {
				added = append(added, c.instances[j])
			}
		}
	}

	// assign to hosts
	errp := placement.Assign(added, sources)
	if errp != nil {
		err = errp
		return
	}

	// cull any instances flagged for removal and enforce upper limits
	for i := range sources {
		for _, instance := range sources[i].Instances() {
			if instance.On == nil && !instance.remove {
				err = errors.New("deployment: one or more instances were not assigned to a host")
				return
			}
		}
		removed = append(removed, sources[i].trimInstances()...)
	}

	// check for basic link consistency and ordering
	links, erro := sources.OrderLinks()
	if erro != nil {
		err = erro
		return
	}

	log.Printf("Order: %+v", links)

	// expose ports for all links
	for i := range links {
		if erre := links[i].exposePorts(); erre != nil {
			err = erre
			return
		}
	}

	// load and reserve all ports
	table, errn := NewInstancePortTable(sources)
	if errn != nil {
		err = errn
		return
	}
	for i := range links {
		if errr := links[i].reserve(table); errr != nil {
			err = errr
			return
		}
	}

	// generate the links
	for i := range links {
		if erra := links[i].appendLinks(); erra != nil {
			err = erra
			return
		}
	}

	// create a copy of instances to return
	instances := make(Instances, 0, len(added))
	for i := range sources {
		existing := sources[i].instances
		for j := range existing {
			instances = append(instances, *existing[j])
		}
	}
	d.Containers = sources
	d.Instances = instances
	next = &d
	return
}

func (d *Deployment) createInstances(c *Container) error {
	for i := len(c.instances); i < c.Count; i++ {
		var id containers.Identifier
		var err error
		if d.RandomizeIds {
			id, err = containers.NewRandomIdentifier(d.IdPrefix)
		} else {
			id, err = containers.NewIdentifier(d.IdPrefix + c.Name + "-" + strconv.Itoa(i+1))
		}
		if err != nil {
			return err
		}
		instance := &Instance{
			Id:    id,
			From:  c.Name,
			Image: c.Image,
			Ports: newPortMappings(c.PublicPorts),

			container: c,
			add:       true,
		}
		c.AddInstance(instance)
	}
	return nil
}

// Invoke to update instance links to the correct external
// ports.
func (d *Deployment) UpdateLinks() {
	for i := range d.Instances {
		instance := &d.Instances[i]
		for j := range instance.links {
			link := &instance.links[j]
		Found:
			for k := range d.Instances {
				ref := &d.Instances[k]
				if ref.From == link.from {
					if assignment, ok := ref.Ports.FindTarget(containers.HostPort{link.FromHost, link.FromPort}); ok {
						if assignment.External != 0 {
							link.ToPort = assignment.External
							break Found
						}
					}
				}
			}
		}
	}
}

// Return a set of container locators from the specified deployment
// descriptor.
func ExtractContainerLocatorsFromDeployment(path string, args *[]string) error {
	if path == "" {
		return nil
	}
	deployment, err := NewDeploymentFromFile(path)
	if err != nil {
		return err
	}
	ids := deployment.Instances.Ids()
	for i := range ids {
		*args = append(*args, ids[i].Identity())
	}
	return nil
}

// A container description
type Container struct {
	Name        string
	Image       string
	PublicPorts containers.PortPairs `json:"PublicPorts,omitempty"`
	Links       Links                `json:"Links,omitempty"`

	Count    int
	Affinity string `json:"Affinity,omitempty"`

	// Instances for this container
	instances InstanceRefs
}
type Containers []Container

func (c *Container) AddInstance(instance *Instance) {
	c.instances = append(c.instances, instance)
}

func (c *Container) Instances() InstanceRefs {
	return c.instances
}

func (c *Container) trimInstances() InstanceRefs {
	count := len(c.instances) - c.Count
	removed := make(InstanceRefs, 0, count)
	remain := make(InstanceRefs, 0, c.Count)
	for i := range c.instances {
		if c.instances[i].On == nil {
			continue
		}
		if c.instances[i].remove {
			removed = append(removed, c.instances[i])
			count--
			continue
		}
		remain = append(remain, c.instances[i])
	}
	if len(remain) > c.Count {
		removed = append(removed, remain[c.Count:]...)
		c.instances = remain[0:c.Count]
	} else {
		c.instances = remain
	}
	return removed
}

func (c Containers) Find(name string) (*Container, bool) {
	for i := range c {
		if c[i].Name == name {
			return &c[i], true
		}
	}
	return nil, false
}

func (c Containers) Copy() (dup Containers) {
	dup = make(Containers, 0, len(c))
	for _, container := range c {
		container.instances = InstanceRefs{}
		links := make(Links, len(container.Links))
		copy(links, container.Links)
		container.Links = links
		dup = append(dup, container)
	}
	return
}
