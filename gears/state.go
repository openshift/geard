package gears

import (
	"io/ioutil"
	"os"
)

func WriteGearState(id Identifier, active bool) error {
	contents := []byte(unitStateContents(id.UnitDefinitionPathFor(), active))
	if err := ioutil.WriteFile(id.UnitPathFor(), contents, 0660); err != nil {
		return err
	}
	return nil
}

func WriteGearStateTo(file *os.File, id Identifier, active bool) error {
	contents := []byte(unitStateContents(id.UnitDefinitionPathFor(), active))
	if _, err := file.Write(contents); err != nil {
		return err
	}
	return nil
}

func unitStateContents(path string, active bool) string {
	var target string
	if active {
		target = "gear-active"
	} else {
		target = "gear"
	}

	return ".include " + path + "\n\n[Install]\nWantedBy=" + target + ".target\n"
}
