package ssh

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/utils"
)

const ContainerPermissionType = "container"

func init() {
	handler := &containerPermission{}
	AddPermissionHandler("", handler)
	AddPermissionHandler(ContainerPermissionType, handler)

	// Register the required configuration directories
	config.AddRequiredDirectory(
		0755,
		config.ContainerBasePath(),
		filepath.Join(config.ContainerBasePath(), "access", "containers", "ssh"),
		filepath.Join(config.ContainerBasePath(), "keys", "public"),
	)
}

type containerPermission struct{}

func (c containerPermission) CreatePermission(locator KeyLocator, value *utils.RawMessage) error {
	var idString string
	if value != nil {
		if err := json.Unmarshal(*value, &idString); err != nil {
			return err
		}
	}

	id, err := containers.NewIdentifier(idString)
	if err != nil {
		return err
	}

	if _, err := os.Stat(id.UnitPathFor()); err != nil {
		return err
	}
	if err := os.Symlink(locator.PathToKey(), SshAccessPathFor(id, locator.NameForKey())); err != nil && !os.IsExist(err) {
		return err
	}
	if _, err := os.Stat(id.AuthKeysPathFor()); err == nil {
		if err := os.Remove(id.AuthKeysPathFor()); err != nil {
			return err
		}
	}
	return nil
}

func SshAccessBasePath(i containers.Identifier) string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "access", "containers", "ssh"), string(i), "", 0775)
}

func SshAccessPathFor(i containers.Identifier, name string) string {
	return utils.IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "access", "containers", "ssh"), string(i), name, 0775)
}
