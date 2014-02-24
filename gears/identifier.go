package gears

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/utils"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
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

func NewIdentifierFromUser(u *user.User) (Identifier, error) {
	id := strings.TrimLeft(u.Username, "gear-")
	return NewIdentifier(id)
}

func (i Identifier) UnitPathFor() string {
	return filepath.Join(config.GearBasePath(), "units", i.UnitNameFor())
}

func (i Identifier) VersionedUnitPathFor(suffix string) string {
	return utils.IsolateContentPath(filepath.Join(config.GearBasePath(), "units"), string(i), suffix)
}

func (i Identifier) LoginFor() string {
	return fmt.Sprintf("gear-%s", i)
}

func (i Identifier) UnitNameFor() string {
	return fmt.Sprintf("gear-%s.service", i)
}

func (i Identifier) UnitNameForJob() string {
	return fmt.Sprintf("job-%s.service", i)
}

func (i Identifier) RepositoryPathFor() string {
	return filepath.Join(config.GearBasePath(), "git", string(i))
}

func (i Identifier) EnvironmentPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.GearBasePath(), "env", "contents"), string(i), "")
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

type JobIdentifier []byte

// An identifier for an individual request
func (j JobIdentifier) UnitNameFor() string {
	return fmt.Sprintf("job-%s.service", safeUnitName(j))
}

func (j JobIdentifier) UnitNameForBuild() string {
	return fmt.Sprintf("build-%s.service", safeUnitName(j))
}

func safeUnitName(b []byte) string {
	return strings.Trim(base64.URLEncoding.EncodeToString(b), "=")
}
