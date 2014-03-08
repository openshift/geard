package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/smarterclayton/geard/containers"
	"log"
	"os"
)

func GenerateId() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

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
		log.Printf("Failed to extract env")
		return err
	}
	e.Description.Variables = append(e.Description.Variables, env...)
	if generateId && !e.Description.Empty() && e.Description.Id == "" {
		e.Description.Id = containers.Identifier(GenerateId())
		log.Printf("Setting --env-id to %s", e.Description.Id)
	}
	return nil
}
