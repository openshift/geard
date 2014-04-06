package idler

import (
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/port"
)

type Port struct {
	port.Port
}

func TcpPort(int port) Port {
	return Port{port.Port(port)}
}

func (p Port) IdentifierFor() (containers.Identifier, error) {
	var id containers.Identifier
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
			if id, err = NewIdentifier(strings.TrimPrefix(line, "X-ContainerId=")); err != nil {
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
