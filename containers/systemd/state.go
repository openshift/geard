package systemd

import (
	"os"
	"path/filepath"

	"github.com/openshift/geard/containers"
)

func activeUnitPathFor(i containers.Identifier) string {
	return filepath.Join("/etc/systemd/system/container-active.target.wants", i.UnitNameFor())
}

func UnitStartOnBoot(i containers.Identifier) (bool, error) {
	if _, err := os.Lstat(activeUnitPathFor(i)); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func SetUnitStartOnBoot(i containers.Identifier, active bool) error {
	if active {
		if err := os.Symlink(i.UnitPathFor(), activeUnitPathFor(i)); err != nil && !os.IsExist(err) {
			return err
		}
	} else {
		if err := os.Remove(activeUnitPathFor(i)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
