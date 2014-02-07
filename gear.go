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

func (g GearIdentifier) UnitPathFor() string {
	return filepath.Join(basePath, "units", g.UnitNameFor())
}

func (g GearIdentifier) UnitNameFor() string {
	return fmt.Sprintf("gear-%s.service", g)
}

func (g GearIdentifier) UnitNameForJob() string {
	return fmt.Sprintf("job-%s.service", g)
}

func (g GearIdentifier) RepositoryPathFor() string {
	return filepath.Join(basePath, "git", string(g))
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
