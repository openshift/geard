// +build idler

package iptables

import (
	"bufio"
	"fmt"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/port"
	"os"
	"strings"
)

type Port struct {
	port.Port
}

func TcpPort(p int) Port {
	return Port{port.Port(p)}
}

func (p Port) IdentifierFor() (containers.Identifier, error) {
	_, portPath := p.PortPathsFor()

	r, err := os.Open(portPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "X-ContainerId=") {
			id, err := containers.NewIdentifier(strings.TrimPrefix(line, "X-ContainerId="))
			if err != nil {
				return "", err
			}
			return id, nil
		}
	}
	if scan.Err() != nil {
		return "", scan.Err()
	}
	return "", fmt.Errorf("Container ID not found")
}
