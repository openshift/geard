package geard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var basePath = "/var/lib/gears"

type Gear string

func VerifyDataPaths() error {
	for _, path := range []string{basePath, filepath.Join(basePath, "units")} {
		if err := checkPath(path, true); err != nil {
			return err
		}
	}
	return nil
}

func UnitPathForGear(id string) string {
	return filepath.Join(basePath, "units", UnitNameForGear(id))
}

func UnitNameForGear(id string) string {
	return fmt.Sprintf("gear-%s.service", id)
}

func checkPath(path string, dir bool) error {
	stat, err := os.Lstat(path)
	if err != nil {
		return errors.New("gear: path (" + path + ") is not available: " + err.Error())
	}
	if stat.IsDir() != dir {
		return errors.New("gear: path (" + path + ") must be a directory instead of a file")
	}
	return nil
}
