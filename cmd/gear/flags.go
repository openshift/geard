package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
	ssh "github.com/openshift/geard/pkg/ssh-public-key"
	"github.com/openshift/geard/port"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
)

func GenerateId() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
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

func ReadAuthorizedKeysFile(keyFile string) ([]jobs.KeyData, error) {

	var (
		data []byte
		keys []jobs.KeyData
		err  error
	)

	// keyFile - contains the sshd AuthorizedKeysFile location
	// Stdin - contains the AuthorizedKeysFile if keyFile is not specified
	if len(keyFile) != 0 {
		absPath, _ := filepath.Abs(keyFile)
		data, err = ioutil.ReadFile(absPath)
		if err != nil {
			return keys, err
		}
	} else {
		data, _ = ioutil.ReadAll(os.Stdin)
	}

	bytesReader := bytes.NewReader(data)
	scanner := bufio.NewScanner(bytesReader)
	for scanner.Scan() {
		// Parse the AuthorizedKeys line
		pk, _, _, _, ok := ssh.ParseAuthorizedKey(scanner.Bytes())
		if !ok {
			err = errors.New("Unable to parse authorized key from input source, invalid format")
			return nil, err
		}
		value := ssh.MarshalAuthorizedKey(pk)
		keys = append(keys, jobs.KeyData{"authorized_keys", string(value)})
	}

	return keys, err
}

func GuessHostIp() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name, "veth") || strings.HasPrefix(iface.Name, "lo") ||
			strings.HasPrefix(iface.Name, "docker") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return ""
		}

		if len(addrs) == 0 {
			continue
		}

		ip, _, _ := net.ParseCIDR(addrs[0].String())
		return ip.String()
	}

	return ""
}
