package containers

import (
	"io/ioutil"
	"os"
)

func WriteContainerState(id Identifier, active bool) error {
	contents := []byte(unitStateContents(id.UnitDefinitionPathFor(), active))
	if err := ioutil.WriteFile(id.UnitPathFor(), contents, 0660); err != nil {
		return err
	}
	return nil
}

func WriteContainerStateTo(file *os.File, id Identifier, active bool) error {
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
