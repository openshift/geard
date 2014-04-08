package containers

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func GetSocketActivation(id Identifier) (bool, string, error) {
	var err error
	var existing *os.File
	if existing, err = os.Open(id.UnitPathFor()); err != nil {
		return false, "disabled", err
	}

	defer existing.Close()
	return readSocketActivationFromUnitFile(existing)
}

func readSocketActivationFromUnitFile(r io.Reader) (bool, string, error) {
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "X-SocketActivated=") {
			sockActStr := strings.TrimPrefix(line, "X-SocketActivated=")
			var val string
			if _, err := fmt.Sscanf(sockActStr, "%s", &val); err != nil {
				return false, "disabled", err
			}
			return val != "disabled", val, nil
		}
	}
	if scan.Err() != nil {
		return false, "disabled", scan.Err()
	}
	return false, "disabled", nil
}
