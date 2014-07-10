package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/port"
)

type VolumeConfig struct {
	*containers.VolumeConfig
}

func (v *VolumeConfig) Get() interface{} {
	return v.VolumeConfig
}

func (v *VolumeConfig) String() string {
	if v.VolumeConfig == nil {
		return ""
	}
	return v.VolumeConfig.String()
}

func (v *VolumeConfig) Set(s string) error {
	volumeConfig, err := containers.VolumeConfigFromString(s)
	if err != nil {
		fmt.Println(os.Stderr, err.Error())
		return err
	}

	v.VolumeConfig = volumeConfig
	return nil
}

type PortPairs struct {
	*port.PortPairs
}

func (p *PortPairs) Get() interface{} {
	if p.PortPairs == nil {
		return &port.PortPairs{}
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
	ports, err := port.FromPortPairHeader(s)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}
	p.PortPairs = &ports
	return nil
}

type NetworkLinks struct {
	*containers.NetworkLinks
}

func (n *NetworkLinks) Get() interface{} {
	return n.NetworkLinks
}

func (n *NetworkLinks) String() string {
	if n.NetworkLinks == nil {
		return ""
	}
	return n.NetworkLinks.ToCompact()
}

func (n *NetworkLinks) Set(s string) error {
	links, err := containers.NewNetworkLinksFromString(s)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}
	n.NetworkLinks = &links
	return nil
}

type EnvironmentDescription struct {
	Description containers.EnvironmentDescription
	Path        string
}

func (e *EnvironmentDescription) ExtractVariablesFrom(args *[]string, generateId bool) error {
	if e.Path != "" {
		file, err := os.Open(e.Path)
		if err != nil {
			return err
		}
		defer file.Close()
		if err := e.Description.ReadFrom(file); err != nil {
			return err
		}
	}
	env, err := containers.ExtractEnvironmentVariablesFrom(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to extract env: "+err.Error())
		return err
	}
	e.Description.Variables = append(e.Description.Variables, env...)
	if generateId && !e.Description.Empty() && e.Description.Id == "" {
		e.Description.Id = containers.Identifier(cmd.GenerateId())
		log.Printf("Setting --env-id to %s", e.Description.Id)
	}
	return nil
}
