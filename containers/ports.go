package containers

import (
	"bufio"
	"github.com/openshift/geard/port"
	"io"
	"os"
	"strings"
)

func GetExistingPorts(id Identifier) (port.PortPairs, error) {
	var existing *os.File
	var err error

	existing, err = os.Open(id.UnitPathFor())
	if err != nil {
		return nil, err
	}
	defer existing.Close()

	return readPortsFromUnitFile(existing)
}

func readPortsFromUnitFile(r io.Reader) (port.PortPairs, error) {
	pairs := make(port.PortPairs, 0, 4)
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "X-PortMapping=") {
			ports := strings.TrimPrefix(line, "X-PortMapping=")
			found, err := port.FromPortPairHeader(ports)
			if err != nil {
				continue
			}
			pairs = append(pairs, found...)
		}
	}
	if scan.Err() != nil {
		return pairs, scan.Err()
	}
	return pairs, nil
}
