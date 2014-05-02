package deployment

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/port"
	"log"
	"sort"
)

// A relationship between two containers
type Link struct {
	To string

	NonLocal  bool `json:"NonLocal,omitempty"`
	MatchPort bool `json:"MatchPort,omitempty"`

	UsePrimary bool `json:"UsePrimary,omitempty"`
	Combine    bool `json:"Combine,omitempty"`

	Ports []port.Port `json:"Ports,omitempty"`

	container *Container
}
type Links []Link

// A materialized link between two instances
type InstanceLink struct {
	containers.NetworkLink

	from     string
	fromPort port.Port
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
				linkedPorts = make([]port.Port, len(target.PublicPorts))
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
	return c[a].priority() > c[b].priority()
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
		p := link.Ports[i]
		for j := range instances {
			target := instances[j]

			_, found := target.Ports.Find(p)
			if !found {
				if _, has := link.Target.PublicPorts.Find(p); !has {
					return errors.New(fmt.Sprintf("deployment: target port %d on %s is not found, cannot link from %s", p, link.Target.Name, link.Source.Name))
				}
				log.Printf("Exposing port %d from target %s so it can be linked", p, target.Id)
				target.Ports = append(
					target.Ports,
					PortMapping{
						port.PortPair{p, port.InvalidPort},
						port.HostPort{"", port.InvalidPort},
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
				if link.NonLocal && mapping.Target.Local() {
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
				//log.Printf("appending %d on %s: %+v %+v", port, instance.Id, mapping, instance)

				name, err := target.ResolveHostname()
				if err != nil {
					return err
				}

				instance.links = append(instance.links, InstanceLink{
					NetworkLink: containers.NetworkLink{
						FromHost: mapping.Target.Host,
						FromPort: mapping.Target.Port,

						ToPort: mapping.External,
						ToHost: name,
					},
					from:     link.Target.Name,
					fromPort: port,
				})
			}
		}
	}
	return nil
}
