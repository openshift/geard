package main

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/containers"
	"net"
)

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
		host[3]++
		if host[3] == 255 {
			host[2]++
			host[3] = 1
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
