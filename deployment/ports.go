package deployment

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/containers"
	"net"
)

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

func (p PortMappings) FindTarget(target containers.HostPort) (*PortMapping, bool) {
	for i := range p {
		if p[i].Target == target {
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

type PortAssignmentStrategy interface {
	Reserve(loopback, same bool, from containers.Port) containers.HostPort
}

type InstancePortTable struct {
	reserved map[containers.HostPort]bool
}

func NewInstancePortTable(sources Containers) (*InstancePortTable, error) {
	table := &InstancePortTable{make(map[containers.HostPort]bool)}

	// make existing reservations
	for i := range sources {
		instances := sources[i].Instances()
		for j := range instances {
			instance := instances[j]
			for k := range instance.Ports {
				target := instance.Ports[k].Target
				if target.Empty() {
					continue
				}
				_, found := table.reserved[target]
				if found {
					return nil, errors.New(fmt.Sprintf("deployment: The port %s is assigned to multiple instances (last: %s)", target.String(), instance.Id))
				}
				table.reserved[target] = true
			}
		}
	}
	return table, nil
}

func (p *InstancePortTable) Reserve(loopback, same bool, from containers.Port) containers.HostPort {
	switch {
	case same && loopback:
		return p.nextHost(net.IPv4(127, 0, 0, 1), from)
	case same:
		return p.nextHost(net.IPv4(192, 168, 1, 1), from)
	case loopback:
		return p.nextPort(net.IPv4(127, 0, 0, 1), from)
	default:
		return p.nextPort(net.IPv4(192, 168, 1, 1), from)
	}
}

func (p *InstancePortTable) nextHost(host net.IP, port containers.Port) containers.HostPort {
	key := containers.HostPort{host.String(), port}
	for {
		if _, ok := p.reserved[key]; !ok {
			p.reserved[key] = true
			return key
		}
		last := len(host) - 1
		host[last]++
		if host[last] == 255 {
			host[last-1]++
			host[last] = 1
		}
		key.Host = host.String()
	}
	panic("Unable to locate a valid host")
}

func (p *InstancePortTable) nextPort(host net.IP, from containers.Port) containers.HostPort {
	key := containers.HostPort{host.String(), 0}
	for port := from; ; port++ {
		key.Port = port
		if _, ok := p.reserved[key]; !ok {
			p.reserved[key] = true
			return key
		}
		if port > 65535 {
			port = 40000
		}
	}
	panic("Unable to locate a valid port")
}
