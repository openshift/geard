package gears

import (
	"bufio"
	"fmt"
	d "github.com/fsouza/go-dockerclient"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/docker"
	"github.com/smarterclayton/geard/selinux"
	"github.com/smarterclayton/geard/utils"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func InitPreStart(dockerSocket string, gearId Identifier, imageName string) error {
	var err error
	var imgInfo *d.Image

	if _, err = user.Lookup(gearId.LoginFor()); err != nil {
		if _, ok := err.(user.UnknownUserError); !ok {
			return err
		}
		if err = createUser(gearId); err != nil {
			return err
		}
	}

	if imgInfo, err = docker.GetImage(dockerSocket, imageName); err != nil {
		return err
	}

	path := path.Join(gearId.HomePath(), "gear-init.sh")
	u, _ := user.Lookup(gearId.LoginFor())
	file, err := utils.OpenFileExclusive(path, 0700)
	if err != nil {
		fmt.Errorf("gear init pre-start: Unable to open script file: ", err)
		return err
	}
	defer file.Close()
	log.Println("Writing gear-init.sh to ", path)

	volumes := make([]string, 0, 10)
	for volPath, _ := range imgInfo.Config.Volumes {
		volumes = append(volumes, volPath)
	}

	gearUser := imgInfo.Config.User
	if gearUser == "" {
		gearUser = "gear"
	}

	if erre := GearInitTemplate.Execute(file, GearInitScript{
		imgInfo.Config.User == "",
		gearUser,
		u.Uid,
		u.Gid,
		strings.Join(imgInfo.Config.Cmd, " "),
		len(volumes) > 0,
		strings.Join(volumes, " "),
	}); erre != nil {
		log.Printf("gear init pre-start: Unable to output template: %+v", erre)
		return erre
	}
	if err := file.Close(); err != nil {
		return err
	}

	return nil
}

func createUser(gearId Identifier) error {
	cmd := exec.Command("/usr/sbin/useradd", gearId.LoginFor(), "-m", "-d", gearId.HomePath(), "-c", "Gear user")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Println(out)
		return err
	}
	selinux.RestoreCon(gearId.HomePath(), true)
	return nil
}

func InitPostStart(dockerSocket string, gearId Identifier) error {
	var u *user.User
	var container *d.Container
	var err error

	if u, err = user.Lookup(gearId.LoginFor()); err != nil {
		return err
	}

	if _, container, err = docker.GetContainer(dockerSocket, gearId.LoginFor(), true); err != nil {
		return err
	}

	if err = generateAuthorizedKeys(gearId, u, container, true); err != nil {
		return err
	}
	return nil
}

func GenerateAuthorizedKeys(dockerSocket string, u *user.User) error {
	var id Identifier
	var container *d.Container
	var err error

	if u.Name != "Gear user" {
		return nil
	}
	if id, err = NewIdentifierFromUser(u); err != nil {
		return err
	}
	if _, container, err = docker.GetContainer(dockerSocket, id.LoginFor(), false); err != nil {
		return err
	}
	if err = generateAuthorizedKeys(id, u, container, false); err != nil {
		return err
	}
	return nil
}

func generateAuthorizedKeys(id Identifier, u *user.User, container *d.Container, forceCreate bool) error {
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

	if err := selinux.RestoreCon(id.BaseHomePath(), true); err != nil {
		return err
	}
	return nil
}

func getContainerPorts(container *d.Container) (string, []string, error) {
	ipAddr := container.NetworkSettings.IPAddress
	ports := []string{}
	for pstr, _ := range container.NetworkSettings.Ports {
		ports = append(ports, strings.Split(string(pstr), "/")[0])
	}

	return ipAddr, ports, nil
}
