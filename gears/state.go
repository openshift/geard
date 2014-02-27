package gears

import (
	"io/ioutil"
)

func WriteGearState(id Identifier, active bool) error {
	var target string
	if active {
		target = "gear-active"
	} else {
		target = "gear"
	}
	data := ".include " + id.UnitDefinitionPathFor() + "\n\n[Install]\nWantedBy=" + target + ".target\n"
	return ioutil.WriteFile(id.UnitPathFor(), []byte(data), 0660)
}
