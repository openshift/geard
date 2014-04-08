package ssh

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/selinux"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
)

func GenerateAuthorizedKeysFor(user *user.User, forceCreate, printToStdOut bool) error {
	if len(authorizedKeysHandlers) == 0 {
		return errors.New("No authorized key file generators have been registered.")
	}
	for _, handler := range authorizedKeysHandlers {
		if handler.MatchesUser(user) {
			return handler.GenerateAuthorizedKeysFile(user, forceCreate, printToStdOut)
		}
	}
	return errors.New(fmt.Sprintf("The specified user %s can't have an authorized key file generated.", user.Name))
}

func init() {
	AddAuthorizedKeyGenerationType(&containerAuthorizedKeys{})
}

type containerAuthorizedKeys struct{}

func (c *containerAuthorizedKeys) MatchesUser(user *user.User) bool {
	return user.Name == "Container user"
}

func (c *containerAuthorizedKeys) GenerateAuthorizedKeysFile(user *user.User, forceCreate, printToStdOut bool) error {
	id, err := containers.NewIdentifierFromUser(user)
	if err != nil {
		return err
	}
	return generateAuthorizedKeys(id, user, forceCreate, printToStdOut)
}

// FIXME: Refactor into separate responsibilities for file creation, templating, and disk access
func generateAuthorizedKeys(id containers.Identifier, u *user.User, forceCreate, printToStdOut bool) error {
	var (
		err      error
		sshKeys  []string
		destFile *os.File
		srcFile  *os.File
		w        *bufio.Writer
	)

	var authorizedKeysPortSpec string
	ports, err := containers.GetExistingPorts(id)
	if err != nil {
		fmt.Errorf("container init pre-start: Unable to retrieve port mapping")
		return err
	}

	for _, port := range ports {
		authorizedKeysPortSpec += fmt.Sprintf("permitopen=\"127.0.0.1:%v\",", port.External)
	}

	sshKeys, err = filepath.Glob(path.Join(SshAccessBasePath(id), "*"))

	if !printToStdOut {
		os.MkdirAll(id.HomePath(), 0700)
		os.Mkdir(path.Join(id.HomePath(), ".ssh"), 0700)
		authKeysPath := id.AuthKeysPathFor()
		if _, err = os.Stat(authKeysPath); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			if forceCreate {
				os.Remove(authKeysPath)
			} else {
				return nil
			}
		}

		if destFile, err = os.Create(authKeysPath); err != nil {
			return err
		}
		defer destFile.Close()
		w = bufio.NewWriter(destFile)
	} else {
		w = bufio.NewWriter(os.Stdout)
	}

	for _, keyFile := range sshKeys {
		s, err := os.Stat(keyFile)
		if err != nil {
			continue
		}
		if s.IsDir() {
			continue
		}

		srcFile, err = os.Open(keyFile)
		defer srcFile.Close()
		w.WriteString(fmt.Sprintf("command=\"%v/bin/switchns\",%vno-agent-forwarding,no-X11-forwarding ", config.ContainerBasePath(), authorizedKeysPortSpec))
		io.Copy(w, srcFile)
		w.WriteString("\n")
	}
	w.Flush()

	if !printToStdOut {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)

		for _, path := range []string{
			id.HomePath(),
			filepath.Join(id.HomePath(), ".ssh"),
			filepath.Join(id.HomePath(), ".ssh", "authorized_keys"),
		} {
			if err := os.Chown(path, uid, gid); err != nil {
				return err
			}
		}

		if err := selinux.RestoreCon(id.BaseHomePath(), true); err != nil {
			return err
		}
	}
	return nil
}
