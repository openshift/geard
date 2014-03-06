package cmd

import (
	"fmt"
	"github.com/smarterclayton/geard/containers"
	"os"
)

type PortPairs struct {
	*containers.PortPairs
}

func (p *PortPairs) Get() interface{} {
	if p.PortPairs == nil {
		return &containers.PortPairs{}
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
	ports, err := containers.FromPortPairHeader(s)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}
	p.PortPairs = &ports
	return nil
}
