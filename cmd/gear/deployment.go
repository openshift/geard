package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"log"
	"os"
	"sort"
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

// A port on a container instance that is linked elsewhere
type PortMapping struct {
	containers.PortPair
	Target containers.HostPort
}
type PortMappings []PortMapping

func newPortMappings(ports containers.PortPairs) PortMappings {
	assignments := make(PortMappings, len(ports))
	for i := range ports {
		assignments[i].PortPair = ports[i]
	}
	return assignments
}

func (p PortMappings) Find(port containers.Port) (*PortMapping, bool) {
	for i := range p {
		if p[i].Internal == port {
			return &p[i], true
		}
	}
	return nil, false
}

func (ports PortMappings) Update(changed containers.PortPairs) bool {
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

func (ports PortMappings) PortPairs() (dup containers.PortPairs) {
	dup = make(containers.PortPairs, len(ports))
	for i := range ports {
		dup[i] = ports[i].PortPair
	}
	return
}

// A relationship between two containers
type Link struct {
	To string

	NonLocal  bool `json:"NonLocal:omitempty"`
	MatchPort bool `json:"PortMatch:omitempty"`

	UsePrimary bool `json:"UsePrimary,omitempty"`
	Combine    bool `json:"Combine,omitempty"`

	Ports []containers.Port `json:"Ports,omitempty"`

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
	if count < 1 {
		return InstanceRefs{}
	}
	removed := make(InstanceRefs, 0, count)
	remain := make(InstanceRefs, 0, c.Count)
	for i := range c.instances {
		if c.instances[i].remove {
			removed = append(removed, c.instances[i])
			count--
			if count < 1 {
				break
			}
			continue
		}
		remain = append(remain, c.instances[i])
	}
	removed = append(removed, remain[c.Count:]...)
	c.instances = remain[0:c.Count]
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
	Ports PortMappings  `json:"Ports,omitempty"`
	Links InstanceLinks `json:"Links,omitempty"`

	// Was this instance added.
	add bool `json:"-"`

	// The container this instance is associated with
	container *Container
	// Is this instance flagged for removal
	remove bool
}
type Instances []Instance
type InstanceRefs []*Instance

type hostnameResolver interface {
	ResolvedHostname() string
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
		if len(refs[i].Links) > 0 {
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

func (d *Deployment) CreateInstances(c *Container) error {
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
	if len(locators) == 0 {
		return nil
	}
	pos := 0
	for i := range added {
		instance := added[i]
		host, _ := cmd.NewHostLocator(locators[pos%len(locators)].HostIdentity())
		instance.On = host
		pos++
	}
	return nil
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
		if errc := d.CreateInstances(c); errc != nil {
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
			if instance.On == nil {
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
	d.Instances = instances
	next = &d
	return
}

func (d *Deployment) UpdateLinks() {
	for i := range d.Instances {
		instance := &d.Instances[i]
		for j := range instance.Links {
			link := &instance.Links[j]
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

// Return the set of links that should be resolved
func (sources Containers) OrderLinks() (ordered containerLinks, err error) {
	links := make(containerLinks, 0)

	for i := range sources { // each container
		source := &sources[i]

		for j := range source.Links { // each link in that container
			link := &source.Links[j]
			target, found := sources.Find(link.To)
			if !found {
				err = errors.New(fmt.Sprintf("deployment: target %s not found for source %s", link.To, source.Name))
				return
			}
			if len(target.PublicPorts) == 0 {
				err = errors.New(fmt.Sprintf("deployment: target %s has no public ports to link to from %s", target.Name, source.Name))
				return
			}
			link.container = target

			// by default, use all target ports if non-specified
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
			links = append(links, containerLink{link, source, target})
		}
	}
	sort.Sort(links)
	ordered = links
	return
}

// Rank order container links by their specificity
func (c *containerLink) priority() int {
	p := 0
	if c.Link.MatchPort {
		p += 4
	}
	if c.Link.NonLocal {
		p += 2
	}
	if c.Source == c.Target {
		p += 1
	}
	return p
}
func (c containerLinks) Less(a, b int) bool {
	return c[a].priority() < c[b].priority()
}
func (c containerLinks) Swap(a, b int) {
	c[a], c[b] = c[b], c[a]
}
func (c containerLinks) Len() int {
	return len(c)
}

type containerLink struct {
	*Link
	Source *Container
	Target *Container
}
type containerLinks []containerLink

func (link containerLink) String() string {
	return fmt.Sprintf("%s-%s", link.Source.Name, link.Target.Name)
}

func (link containerLink) exposePorts() error {
	instances := link.Target.Instances()
	for i := range link.Ports {
		port := link.Ports[i]
		for j := range instances {
			target := instances[j]

			_, found := target.Ports.Find(port)
			if !found {
				if _, has := link.Target.PublicPorts.Find(port); !has {
					return errors.New(fmt.Sprintf("deployment: target port %d on %s is not found, cannot link from %s", port, link.Target.Name, link.Source.Name))
				}
				log.Printf("Exposing port %d from target %s so it can be linked", port, target.Id)
				target.Ports = append(
					target.Ports,
					PortMapping{
						containers.PortPair{port, containers.InvalidPort},
						containers.HostPort{"", containers.InvalidPort},
					},
				)
			}
		}
	}
	return nil
}

func (link containerLink) reserve(pool PortAssignmentStrategy) error {
	instances := link.Target.Instances()
	for i := range link.Ports {
		port := link.Ports[i]
		for j := range instances {
			instance := instances[j]
			mapping, found := instance.Ports.Find(port)
			if !found {
				return errors.New(fmt.Sprintf("deployment: instance does not expose %d for link %s", port, link.String()))
			}

			if !mapping.Target.Empty() {
				if link.NonLocal && !mapping.Target.Local() {
					return errors.New(fmt.Sprintf("deployment: A local host IP is already bound to non-local link %s, needs to be reset.", link.String()))
				}
				if link.MatchPort && mapping.Target.Port != mapping.Internal {
					return errors.New(fmt.Sprintf("deployment: The internal and shared ports are not the same for an instance %s on link %s, needs to be reset.", instance.Id, link.String()))
				}
				continue
			}
			mapping.Target = pool.Reserve(!link.NonLocal, link.MatchPort, port)
		}
	}
	return nil
}

func (link containerLink) appendLinks() error {
	targetInstances := link.Target.Instances()
	sourceInstances := link.Source.Instances()

	for i := range sourceInstances {
		instance := sourceInstances[i]
		for j := range targetInstances {
			target := targetInstances[j]
			for k := range link.Ports {
				port := link.Ports[k]
				mapping, found := target.Ports.Find(port)
				if !found {
					return errors.New(fmt.Sprintf("deployment: instance does not expose %d for link %s", port, link.String()))
				}

				instance.Links = append(instance.Links, InstanceLink{
					NetworkLink: containers.NetworkLink{
						FromHost: mapping.Target.Host,
						FromPort: mapping.Target.Port,

						ToPort: mapping.External,
						ToHost: instance.ResolvedHostname(),
					},
					from:     link.Target.Name,
					fromPort: port,
				})
			}
		}
	}
	return nil
}
