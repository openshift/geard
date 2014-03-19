package containers

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func WriteContainerState(id Identifier, active bool) error {
	contents := []byte(unitStateContents(id.UnitDefinitionPathFor(), active))
	if err := ioutil.WriteFile(id.UnitPathFor(), contents, 0660); err != nil {
		return err
	}
	return nil
}

func WriteContainerStateTo(file *os.File, id Identifier, active bool) error {
	if err := file.Truncate(0); err != nil {
		return err
	}

	contents := []byte(unitStateContents(id.UnitDefinitionPathFor(), active))
	if _, err := file.Write(contents); err != nil {
		return err
	}
	return nil
}

func unitStateContents(path string, active bool) string {
	var target string
	if active {
		target = "container-active"
	} else {
		target = "container"
	}

	return ".include " + path + "\n\n[Install]\nWantedBy=" + target + ".target\n"
}

func ReadContainerState(id Identifier) (bool, error) {
	r, err := os.Open(id.UnitPathFor())
	if err != nil {
		return false, err
	}
	defer r.Close()

	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "WantedBy=") {
			wantedBy := strings.TrimPrefix(line, "WantedBy=")
			return strings.Contains(wantedBy, "container-active.target"), nil
		}
	}
	if scan.Err() != nil {
		return false, scan.Err()
	}
	return false, fmt.Errorf("Container state not found")
}
