package containers

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/utils"
)

type Identifier string

const IdentifierPrefix = "ctr-"
const IdentifierSuffixPattern = "[a-zA-Z0-9\\-\\.]{4,24}"

var InvalidIdentifier = Identifier("")
var allowedIdentifier = regexp.MustCompile("\\A" + IdentifierSuffixPattern + "\\z")

func NewIdentifier(s string) (Identifier, error) {
	switch {
	case s == "":
		return InvalidIdentifier, errors.New("Identifier may not be empty")
	case !allowedIdentifier.MatchString(s):
		return InvalidIdentifier, errors.New("Identifier must match " + allowedIdentifier.String())
	}
	return Identifier(s), nil
}

func NewRandomIdentifier(prefix string) (Identifier, error) {
	i := make([]byte, (32-4-len(prefix))*4/3)
	if _, err := rand.Read(i); err != nil {
		return InvalidIdentifier, err
	}
	return Identifier(strings.TrimRight(base64.URLEncoding.EncodeToString(i), "=")), nil
}

func NewIdentifierFromUser(u *user.User) (Identifier, error) {
	if !strings.HasPrefix(u.Username, IdentifierPrefix) || u.Name != "Container user" {
		return InvalidIdentifier, errors.New("Not a container user")
	}
	id := strings.TrimPrefix(u.Username, IdentifierPrefix)
	return NewIdentifier(id)
}

func (i Identifier) UnitPathFor() string {
	base := utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "units"), string(i), "", 0775)
	return filepath.Join(filepath.Dir(base), i.UnitNameFor())
}

func (i Identifier) IdleUnitPathFor() string {
	base := utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "units"), string(i), "", 0775)
	return filepath.Join(filepath.Dir(base), i.UnitIdleFlagNameFor())
}

func (i Identifier) VersionedUnitsPathFor() string {
	return i.VersionedUnitPathFor("")
}

func (i Identifier) VersionedUnitPathFor(suffix string) string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "units"), string(i), suffix, 0775)
}

func (i Identifier) UnitNameFor() string {
	return fmt.Sprintf("%s%s.service", IdentifierPrefix, i)
}

func (i Identifier) UnitIdleFlagNameFor() string {
	return fmt.Sprintf("%s%s.idle", IdentifierPrefix, i)
}

func (i Identifier) SocketUnitPathFor() string {
	base := utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "units"), string(i), "", 0775)
	return filepath.Join(filepath.Dir(base), i.SocketUnitNameFor())
}

func (i Identifier) SocketUnitNameFor() string {
	return fmt.Sprintf("%s%s.socket", IdentifierPrefix, i)
}

func (i Identifier) LoginFor() string {
	return fmt.Sprintf("%s%s", IdentifierPrefix, i)
}

func (i Identifier) UnitNameForJob() string {
	return fmt.Sprintf("job-%s.service", i)
}

func (i Identifier) EnvironmentPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "env", "contents"), string(i), "")
}

func (i Identifier) NetworkLinksPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "ports", "links"), string(i), "")
}

func (i Identifier) BaseHomePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "home"), string(i), "", 0775)
}

func (i Identifier) HomePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "home"), string(i), "home", 0775)
}

func (i Identifier) RunPathFor() string {
	return utils.IsolateContentPathWithPerm(config.ContainerRunPath(), string(i), "/", 0775)
}

func (i Identifier) AuthKeysPathFor() string {
	return filepath.Join(i.HomePath(), ".ssh", "authorized_keys")
}

func (i Identifier) PortDescriptionPathFor() string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "ports", "descriptions"), string(i), "")
}

func (i Identifier) ContainerFor() string {
	return fmt.Sprintf("%s", i)
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
