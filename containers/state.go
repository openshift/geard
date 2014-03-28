package containers

import (
	"os"
	"path/filepath"
)

func (i Identifier) activeUnitPathFor() string {
	return filepath.Join("/etc/systemd/system/container-active.target.wants", i.UnitNameFor())
}

func (i Identifier) UnitStartOnBoot() (bool, error) {
	if _, err := os.Lstat(i.activeUnitPathFor()); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (i Identifier) SetUnitStartOnBoot(active bool) error {
	if active {
		if err := os.Symlink(i.UnitPathFor(), i.activeUnitPathFor()); err != nil && !os.IsExist(err) {
			return err
		}
	} else {
		if err := os.Remove(i.activeUnitPathFor()); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
