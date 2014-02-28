package gears

import (
	"io/ioutil"
)

func WriteGearState(id Identifier, active bool, socketActivated bool) error {
	var gearTarget string
	var sockTarget string
	if active {
		if socketActivated {
			sockTarget = "gear-active"
			gearTarget = "gear"
		} else {
			gearTarget = "gear-active"
		}
	} else {
		sockTarget = "gear"
		gearTarget = "gear"
	}

	data := ".include " + id.UnitDefinitionPathFor() + "\n\n[Install]\nWantedBy=" + gearTarget + ".target\n"
	if err := ioutil.WriteFile(id.UnitPathFor(), []byte(data), 0660); err != nil {
		return err
	}

	if socketActivated {
		data := ".include " + id.SocketUnitDefinitionPathFor() + "\n\n[Install]\nWantedBy=" + sockTarget + ".target\n"
		return ioutil.WriteFile(id.SocketUnitPathFor(), []byte(data), 0660)
	}
	return nil
}
