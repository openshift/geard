package cmd

import (
	"github.com/smarterclayton/geard/gears"
)

type PortPairs struct {
	*gears.PortPairs
}

func (p *PortPairs) Get() interface{} {
	if p.PortPairs == nil {
		return &gears.PortPairs{}
	}
	return p.PortPairs
}

func (p *PortPairs) String() string {
	if p.PortPairs == nil {
		return ""
	}
	return p.PortPairs.ToHeader()
}

func (p *PortPairs) Set(s string) error {
	ports, err := gears.FromPortPairHeader(s)
	if err != nil {
		return err
	}
	p.PortPairs = &ports
	return nil
}
