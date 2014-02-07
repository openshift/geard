package geard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type GearIdentifier string

var InvalidGearIdentifier = GearIdentifier("")
var allowedGearIdentifier = regexp.MustCompile("\\A[a-fA-F0-9]{4,32}\\z")

func NewGearIdentifier(s string) (GearIdentifier, error) {
	switch {
	case s == "":
		return InvalidGearIdentifier, errors.New("Gear identifier may not be empty")
	case !allowedGearIdentifier.MatchString(s):
		return InvalidGearIdentifier, errors.New("Gear identifier must match " + allowedGearIdentifier.String())
	}
	return GearIdentifier(s), nil
}

var basePath = "/var/lib/gears"

func VerifyDataPaths() error {
	for _, path := range []string{basePath, filepath.Join(basePath, "units")} {
		if err := checkPath(path, true); err != nil {
			return err
		}
	}
	return nil
}

func UnitPathForGear(id GearIdentifier) string {
	return filepath.Join(basePath, "units", UnitNameForGear(id))
}

func UnitNameForGear(id GearIdentifier) string {
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
