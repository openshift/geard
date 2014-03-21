package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/cmd"
	"github.com/smarterclayton/geard/containers"
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

// A port and how it is seen in different contexts
type PortAssignment struct {
	containers.PortPair
	Shared containers.Port `json:"Shared,omitempty"`
}
type PortAssignments []PortAssignment

func (p PortAssignments) Find(port containers.Port) (*PortAssignment, bool) {
	for i := range p {
		if p[i].Internal == port {
			return &p[i], true
		}
	}
	return nil, false
}

func (ports PortAssignments) Update(changed containers.PortPairs) bool {
	matched := true
	for i := range ports {
		port := &ports[i]
	NextPort:
		for j := range changed {
			if port.Internal == changed[j].Internal {
				port.External = changed[j].External
				break NextPort
			}
		}
		matched = true
	}
	return matched
}

func (ports PortAssignments) PortPairs() (dup containers.PortPairs) {
	dup = make(containers.PortPairs, len(ports))
	for i := range ports {
		dup[i] = ports[i].PortPair
	}
	return
}

// A relationship between two containers
type Link struct {
	To         string
	UsePrimary bool              `json:"UsePrimary,omitempty"`
	Ports      []containers.Port `json:"Ports,omitempty"`
	Combine    bool              `json:"Combine,omitempty"`

	container *Container
}
type Links []Link

// A materialized link between two instances
type InstanceLink struct {
	containers.NetworkLink

	from     string
	fromPort containers.Port
	matched  bool
}
type InstanceLinks []InstanceLink

func (links InstanceLinks) NetworkLinks() (dup containers.NetworkLinks) {
	dup = make(containers.NetworkLinks, len(links))
	for i := range links {
		dup[i] = links[i].NetworkLink
	}
	return
}

// Definition of a container
type Container struct {
	Name        string
	Image       string
	PublicPorts containers.PortPairs `json:"PublicPorts,omitempty"`
	Links       Links                `json:"Links,omitempty"`

	Count    int
	Affinity string `json:"Affinity,omitempty"`

	// Number of instances known at this time
	found int
}
type Containers []Container

func (d *Deployment) CreateInstances(c *Container) ([]Instance, error) {
	instances := []Instance{}
	for i := c.found; i < c.Count; i++ {
		var id containers.Identifier
		var err error
		if d.RandomizeIds {
			id, err = containers.NewRandomIdentifier(d.IdPrefix)
		} else {
			id, err = containers.NewIdentifier(d.IdPrefix + c.Name + "-" + strconv.Itoa(i+1))
		}
		if err != nil {
			return []Instance{}, err
		}
		instances = append(instances, Instance{
			Id:        id,
			From:      c.Name,
			Image:     c.Image,
			Ports:     newPortAssignments(c.PublicPorts),
			container: c,
		})
	}
	return instances, nil
}

func newPortAssignments(ports containers.PortPairs) PortAssignments {
	assignments := make(PortAssignments, len(ports))
	for i := range ports {
		assignments[i].PortPair = ports[i]
	}
	return assignments
}

func (c *Container) Over() bool {
	return c.found > c.Count
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
		container.found = 0
		links := make(Links, len(container.Links))
		copy(links, container.Links)
		container.Links = links
		dup = append(dup, container)
	}
	return
}

// A container that has been created on a server
type Instance struct {
	// The id of this instance
	Id containers.Identifier
	// The image used by this instance
	Image string
	// The container definition this is from
	From string
	// The host system this is or should be deployed on
	On    *cmd.HostLocator `json:"On,omitempty"`
	Ports PortAssignments  `json:"Ports,omitempty"`
	Links InstanceLinks    `json:"Links,omitempty"`

	// Was this instance added.
	Add bool `json:"Add,omitempty"`

	// The container this instance is associated with
	container *Container
	// Is this instance flagged for removal
	remove bool
}
type Instances []Instance

type hostnameResolver interface {
	ResolvedHostname() string
}

func (i *Instance) ResolvedHostname() string {
	return i.On.ResolvedHostname()
}

func (i *Instance) EnvironmentVariables() {

}

func (inst *Instance) EnsureReserved(ports portReservation) {
	for j := range inst.Ports {
		port := &inst.Ports[j]
		if port.Shared == 0 {
			port.Shared = ports.ReserveFrom(port.Internal, inst)
		}
	}
}

func (instances Instances) ReservePorts(ports portReservation) {
	for i := range instances {
		instance := &instances[i]
		for j := range instance.Ports {
			s := instance.Ports[j].Shared
			if s != 0 {
				ports.Reserve(s, instance)
			}
		}
	}
}

func (instances Instances) Find(id containers.Identifier) (*Instance, bool) {
	for i := range instances {
		if string(instances[i].Id) == string(id) {
			return &instances[i], true
		}
	}
	return nil, false
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
		if refs[i].Add {
			ids = append(ids, &cmd.ContainerLocator{*refs[i].On, refs[i].Id})
		}
	}
	return
}

func (refs Instances) LinkedIds() (ids []cmd.Locator) {
	ids = make([]cmd.Locator, 0, len(refs))
	for i := range refs {
		if len(refs[i].Links) > 0 {
			ids = append(ids, &cmd.ContainerLocator{*refs[i].On, refs[i].Id})
		}
	}
	return
}

type InstanceRefs []*Instance

func (refs InstanceRefs) Ids() (ids []cmd.Locator) {
	ids = make([]cmd.Locator, 0, len(refs))
	for i := range refs {
		if refs[i] != nil {
			ids = append(ids, &cmd.ContainerLocator{*refs[i].On, refs[i].Id})
		}
	}
	return
}

type portReservation map[containers.Port]*Instance

func newPortReservation() portReservation {
	return make(portReservation)
}

func (p portReservation) ReserveFrom(from containers.Port, instance *Instance) (port containers.Port) {
	for port = from; ; port++ {
		if _, ok := p[port]; !ok {
			p[port] = instance
			return
		}
		if port > 65535 {
			port = 40000
		}
	}
	return
}

func (p portReservation) Reserve(port containers.Port, instance *Instance) bool {
	if _, found := p[port]; !found {
		p[port] = instance
		return true
	}
	return false
}

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

func (d Deployment) Describe(on cmd.Locators) (next *Deployment, removed InstanceRefs, err error) {
	if len(on) == 0 {
		err = errors.New("one or more hosts required")
		return
	}

	// reset the container list
	from := d.Containers.Copy()

	// check existing instances and flag any that need to be removed
	existing := make(Instances, 0, len(d.Instances))
	for _, inst := range d.Instances {
		// is the host available now
		if inst.On == nil {
			continue
		}
		inst.remove = !on.Has(inst.On)
		// locate the container
		if c, found := from.Find(inst.From); found {
			inst.container = c
			c.found++
		}
		existing = append(existing, inst)
	}

	// remove any instances that should no longer be present
	valid := make(Instances, 0, len(d.Instances))
	for i := range existing {
		inst := &existing[i]
		if inst.container == nil || inst.container.Over() || inst.remove {
			if inst.container != nil {
				inst.container.found--
			}
			removed = append(removed, inst)
			continue
		}
		valid = append(valid, *inst)
	}

	// create new instances for each container
	for i := range from {
		c := &from[i]
		new, errc := d.CreateInstances(c)
		if errc != nil {
			err = errc
			return
		}
		valid = append(valid, new...)
	}

	// assign to hosts and ensure all ports have been reserved
	reservedPorts := newPortReservation()
	valid.ReservePorts(reservedPorts)
	pos := 0
	for i := range valid {
		inst := &valid[i]
		if inst.On == nil {
			inst.Add = true
			host, _ := cmd.NewHostLocator(on[pos%len(on)].HostIdentity())
			inst.On = host
			pos++
		}
		inst.EnsureReserved(reservedPorts)
		inst.Links = InstanceLinks{}
	}

	for i := range from { // each container
		source := &from[i]

		for j := range source.Links { // each link in that container
			link := &source.Links[j]
			target, found := from.Find(link.To)
			if !found {
				err = errors.New(fmt.Sprintf("deployment: target %s not found for source %s", link.To, source.Name))
				return
			}
			if len(target.PublicPorts) == 0 {
				err = errors.New(fmt.Sprintf("deployment: target %s has no public ports to link to from %s", target.Name, source.Name))
				return
			}
			link.container = target

			// by default, use all target ports if non specified
			linkedPorts := link.Ports
			if len(linkedPorts) == 0 {
				linkedPorts = make([]containers.Port, len(target.PublicPorts))
				for k := range target.PublicPorts {
					linkedPorts[k] = target.PublicPorts[k].Internal
				}
				link.Ports = linkedPorts
			}
			if len(linkedPorts) == 0 {
				err = errors.New(fmt.Sprintf("deployment: target %s has no public ports", target.Name))
				return
			}

			// the things that will be linked to
			targetInstances := make([]*Instance, 0, target.Count)
			for k := range valid {
				if valid[k].From == target.Name {
					targetInstances = append(targetInstances, &valid[k])
				}
			}

			for _, p := range linkedPorts { // each port that is linked
				// find or add a network link on each instance in this container
				for k := range valid {
					instance := &valid[k]
					if instance.From != source.Name {
						continue
					}

					for l := range targetInstances {
						targetInstance := targetInstances[l]
						port, found := targetInstance.Ports.Find(p)
						if !found {
							if _, has := target.PublicPorts.Find(p); !has {
								err = errors.New(fmt.Sprintf("deployment: target port %d on %s is not found, cannot link from %s", p, target.Name, source.Name))
								return
							}
							log.Printf("Exposing port %d from target %s so it can be linked", p, targetInstance.Id)
							// expose the port - TBD whether this should be required to be explicit
							port = &PortAssignment{containers.PortPair{p, containers.Port(0)}, reservedPorts.ReserveFrom(p, targetInstance)}
							targetInstance.Ports = append(targetInstance.Ports, *port)
						}

						//log.Printf("Linking target %v to %v", targetInstance, instance)
						instance.Links = append(instance.Links, InstanceLink{
							NetworkLink: containers.NetworkLink{
								FromPort: port.Shared,
								ToPort:   port.External,
								ToHost:   instance.ResolvedHostname(),
							},
							from:     target.Name,
							fromPort: p,
						})
					}
				}
			}
		}
	}

	d.Instances = valid
	next = &d
	return
}

func (d *Deployment) UpdateLinks() {
	for i := range d.Instances {
		inst := &d.Instances[i]
		for j := range inst.Links {
			link := &inst.Links[j]
		Found:
			for k := range d.Instances {
				ref := &d.Instances[k]
				if ref.From == link.from {
					if assignment, ok := ref.Ports.Find(link.FromPort); ok {
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

type AntiAffinityDeploymentStrategy struct {
}
