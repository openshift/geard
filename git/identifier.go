package git

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/utils"
	"os/user"
	"path/filepath"
	"strings"
)

type RepoIdentifier containers.Identifier

const ResourceTypeRepository = "repo"
const RepoIdentifierPrefix = "git-"

func NewIdentifierFromUser(u *user.User) (RepoIdentifier, error) {
	if !strings.HasPrefix(u.Username, RepoIdentifierPrefix) || u.Name != "Repository user" {
		return "", errors.New("Not a repository user")
	}
	id := strings.TrimLeft(u.Username, RepoIdentifierPrefix)
	containerId, err := containers.NewIdentifier(id)
	if err != nil {
		return "", err
	}
	return RepoIdentifier(containerId), nil
}

func (i RepoIdentifier) UnitPathFor() string {
	base := utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "units"), string(i), "")
	return filepath.Join(filepath.Dir(base), i.UnitNameFor())
}

func (i RepoIdentifier) UnitNameFor() string {
	return fmt.Sprintf("%s%s.service", RepoIdentifierPrefix, i)
}

func (i RepoIdentifier) LoginFor() string {
	return fmt.Sprintf("%s%s", RepoIdentifierPrefix, i)
}

func (i RepoIdentifier) BaseHomePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), fmt.Sprintf("%shome", RepoIdentifierPrefix)), string(i), "", 0775)
}

func (i RepoIdentifier) HomePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), fmt.Sprintf("%shome", RepoIdentifierPrefix)), string(i), "home", 0775)
}

func (i RepoIdentifier) RepositoryPathFor() string {
	return filepath.Join(config.ContainerBasePath(), "git", string(i))
}

func (i RepoIdentifier) GitAccessPathFor(name string, write bool) string {
	var access string
	if write {
		access = ".write"
	} else {
		access = ".read"
	}
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "access", "git"), string(i), name+access, 0775)
}

func (i RepoIdentifier) SshAccessBasePath() string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "access", "git"), string(i), "", 0775)
}

func (i RepoIdentifier) AuthKeysPathFor() string {
	return filepath.Join(i.HomePath(), ".ssh", "authorized_keys")
}
