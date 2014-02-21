package util

import (
	"bufio"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/selinux"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func getContainerPorts(container *docker.Container) (string, []string, error) {
	ipAddr := container.NetworkSettings.IPAddress
	ports := []string{}
	for pstr, _ := range container.NetworkSettings.Ports {
		ports = append(ports, strings.Split(string(pstr), "/")[0])
	}

	return ipAddr, ports, nil
}

func GenerateAuthorizedKeys(id gears.Identifier, u *user.User, container *docker.Container, forceCreate bool) error {
	var err error
	var sshKeys []string
	var destFile *os.File
	var srcFile *os.File

	var authorizedKeysPortSpec string
	if ipAddr, ports, err := getContainerPorts(container); err != nil {
		return err
	} else {
		for _, p := range ports {
			authorizedKeysPortSpec += fmt.Sprintf("permitopen=\"%v:%v\",", ipAddr, p)
		}
	}

	sshKeys, err = filepath.Glob(path.Join(id.SshAccessBasePath(), "*"))
	os.MkdirAll(id.HomePath(), 0700)
	os.Mkdir(path.Join(id.HomePath(), ".ssh"), 0700)
	authKeysPath := path.Join(id.HomePath(), ".ssh", "authorized_keys")
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
	w := bufio.NewWriter(destFile)

	for _, keyFile := range sshKeys {
		s, _ := os.Stat(keyFile)
		if s.IsDir() {
			continue
		}

		srcFile, err = os.Open(keyFile)
		defer srcFile.Close()
		w.WriteString(fmt.Sprintf("command=\"%v/bin/switchns\",%vno-agent-forwarding,no-X11-forwarding ", config.GearBasePath(), authorizedKeysPortSpec))
		io.Copy(w, srcFile)
		w.WriteString("\n")
	}
	w.Flush()

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

	if err := selinux.RestoreConRecursive(id.BaseHomePath()); err != nil {
		return err
	}
	return nil
}
