package containers

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
var allowedIdentifier = regexp.MustCompile("\\A[a-zA-Z0-9\\-\\.]{4,32}\\z")

func NewIdentifier(s string) (Identifier, error) {
	switch {
	case s == "":
		return InvalidIdentifier, errors.New("Identifier may not be empty")
	case !allowedIdentifier.MatchString(s):
		return InvalidIdentifier, errors.New("Identifier must match " + allowedIdentifier.String())
	}
	return Identifier(s), nil
}

func NewIdentifierFromUser(u *user.User) (Identifier, error) {
	id := strings.TrimLeft(u.Username, "container-")
	return NewIdentifier(id)
}

func (i Identifier) UnitPathFor() string {
	base := utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "units"), string(i), "")
	return filepath.Join(filepath.Dir(base), i.UnitNameFor())
}

func (i Identifier) UnitDefinitionPathFor() string {
	return i.VersionedUnitPathFor("definition")
}

func (i Identifier) VersionedUnitPathFor(suffix string) string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "units"), string(i), suffix)
}

func (i Identifier) UnitNameFor() string {
	return fmt.Sprintf("container-%s.service", i)
}

func (i Identifier) SocketUnitPathFor() string {
	base := utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "units"), string(i), "")
	return filepath.Join(filepath.Dir(base), i.SocketUnitNameFor())
}

func (i Identifier) SocketUnitNameFor() string {
	return fmt.Sprintf("container-%s.socket", i)
}

func (i Identifier) LoginFor() string {
	return fmt.Sprintf("container-%s", i)
}

func (i Identifier) UnitNameForJob() string {
	return fmt.Sprintf("job-%s.service", i)
}

func (i Identifier) RepositoryPathFor() string {
	return filepath.Join(config.ContainerBasePath(), "git", string(i))
}

func (i Identifier) EnvironmentPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "env", "contents"), string(i), "")
}

func (i Identifier) NetworkLinksPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "ports", "links"), string(i), "")
}

func (i Identifier) GitAccessPathFor(f utils.Fingerprint, write bool) string {
	var access string
	if write {
		access = ".write"
	} else {
		access = ".read"
	}
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "access", "git"), string(i), f.ToShortName()+access)
}

func (i Identifier) SshAccessBasePath() string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "access", "containers", "ssh"), string(i), "")
}

func (i Identifier) SshAccessPathFor(f utils.Fingerprint) string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "access", "containers", "ssh"), string(i), f.ToShortName())
}

func (i Identifier) BaseHomePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "home"), string(i), "", 0775)
}

func (i Identifier) HomePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "home"), string(i), "home", 0775)
}

func (i Identifier) AuthKeysPathFor() string {
	return filepath.Join(i.HomePath(), ".ssh", "authorized_keys")
}

func (i Identifier) PortDescriptionPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "ports", "descriptions"), string(i), "")
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
