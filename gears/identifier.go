package gears

import (
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/utils"
	"path/filepath"
	"regexp"
)

type Identifier string

var InvalidIdentifier = Identifier("")
var allowedIdentifier = regexp.MustCompile("\\A[a-zA-Z0-9\\-]{4,32}\\z")

func NewIdentifier(s string) (Identifier, error) {
	switch {
	case s == "":
		return InvalidIdentifier, errors.New("Gear identifier may not be empty")
	case !allowedIdentifier.MatchString(s):
		return InvalidIdentifier, errors.New("Gear identifier must match " + allowedIdentifier.String())
	}
	return Identifier(s), nil
}

func (g Identifier) UnitPathFor() string {
	return filepath.Join(config.GearBasePath(), "units", g.UnitNameFor())
}

func (g Identifier) UnitNameFor() string {
	return fmt.Sprintf("gear-%s.service", g)
}

func (g Identifier) UnitNameForJob() string {
	return fmt.Sprintf("job-%s.service", g)
}

func (g Identifier) RepositoryPathFor() string {
	return filepath.Join(config.GearBasePath(), "git", string(g))
}

func (g Identifier) EnvironmentPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.GearBasePath(), "env", "contents"), string(g), "")
}

func (i Identifier) GitAccessPathFor(f utils.Fingerprint, write bool) string {
	var access string
	if write {
		access = ".write"
	} else {
		access = ".read"
	}
	return utils.IsolateContentPath(filepath.Join(config.GearBasePath(), "access", "git"), string(i), f.ToShortName()+access)
}

func (i Identifier) SshAccessBasePath() string {
	return utils.IsolateContentPath(filepath.Join(config.GearBasePath(), "access", "gears", "ssh"), string(i), "")
}

func (i Identifier) SshAccessPathFor(f utils.Fingerprint) string {
	return utils.IsolateContentPath(filepath.Join(config.GearBasePath(), "access", "gears", "ssh"), string(i), f.ToShortName())
}

func (i Identifier) BaseHomePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.GearBasePath(), "home"), string(i), "", 0775)
}

func (i Identifier) HomePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.GearBasePath(), "home"), string(i), "home", 0775)
}

func (i Identifier) PortDescriptionPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.GearBasePath(), "ports", "descriptions"), string(i), "")
}
