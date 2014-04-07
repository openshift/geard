package containers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"strconv"

	"github.com/openshift/geard/docker"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/selinux"
	"github.com/openshift/geard/utils"

	dc "github.com/fsouza/go-dockerclient"
)

type ContainerData struct {
	User            string
	Uid             string
	Gid             string
	Command         []string
	Volumes         []string
	Ports           port.PortPairs
	SocketActivated bool
	Links           []NetworkLink
}

func GetContainerData(dockerSocket string, id Identifier, imageName string, createMissingUser bool) (data *ContainerData, err error) {
	var (
		socketActivationType string
		ports                port.PortPairs
		d                    *docker.DockerClient
		imgInfo              *dc.Image
		linkReader           io.Reader
		u                    *user.User
		hostIp               net.IP
	)

	_, socketActivationType, err = GetSocketActivation(id)
	if err != nil {
		fmt.Printf("init_pre_start: Error while parsing unit file: %v\n", err)
		return
	}

	if d, err = docker.GetConnection(dockerSocket); err != nil {
		return
	}

	if imgInfo, err = d.GetImage(imageName); err != nil {
		return
	}

	u, err = user.Lookup(id.LoginFor())
	if err != nil {
		if !createMissingUser {
			return
		} else if err = createUser(id); err != nil {
			return
		}

		if u, err = user.Lookup(id.LoginFor()); err != nil {
			return
		}
	}
	volumes := make([]string, 0, 10)
	for volPath := range imgInfo.Config.Volumes {
		volumes = append(volumes, volPath)
	}

	ports, err = GetExistingPorts(id)
	if err != nil {
		fmt.Printf("container init pre-start: Unable to retrieve port mapping\n")
		return
	}

	links := []NetworkLink{}
	if linkReader, err = os.Open(id.NetworkLinksPathFor()); err == nil {
		for {
			link := NetworkLink{}
			if _, err := fmt.Fscanf(linkReader, "%s\t%v\t%v\t%s\n", &link.FromHost, &link.FromPort, &link.ToPort, &link.ToHost); err != nil {
				if err == io.EOF {
					break
				}
				fmt.Printf("gear: Could not read from network links file: %v\n", err)
				break
			}
			if hostIp, err = utils.Resolver.ResolveIP(link.ToHost); err != nil {
				return
			}
			link.ToHost = hostIp.String()
			links = append(links, link)
		}
	} else {
		fmt.Println(err)
	}

	data = &ContainerData{
		User:            imgInfo.Config.User,
		Uid:             u.Uid,
		Gid:             u.Gid,
		Command:         imgInfo.Config.Cmd,
		Volumes:         volumes,
		Ports:           ports,
		SocketActivated: socketActivationType == "proxied",
		Links:           links,
	}
	return
}

func UpdateInitEnvironment(dockerSocket string, id Identifier, imageName string) (err error) {
	var (
		data        *ContainerData
		encodedData []byte
		file        *os.File
	)

	if data, err = GetContainerData(dockerSocket, id, imageName, true); err != nil {
		return
	}

	if encodedData, err = json.Marshal(data); err != nil {
		return
	}

	envPath := filepath.Join(id.RunPathFor(), "container-init.env")
	file, err = os.OpenFile(envPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0660)
	if os.IsExist(err) {
		file, err = os.OpenFile(envPath, os.O_TRUNC|os.O_WRONLY, 0660)
	}
	if _, err = fmt.Fprintf(file, "GEARD_CONTAINER_INFO=%s\n", strconv.Quote(string(encodedData))); err != nil {
		log.Print("Unable to write to container-init environment file: ", err)
		return
	}
	if err := file.Close(); err != nil {
		log.Print("Unable to close container-init environment file: ", err)
		return err
	}
	return
}

func CreateContainerInitScripts(dockerSocket string, id Identifier, imageName string) (err error) {
	var (
		data *ContainerData
	)

	if data, err = GetContainerData(dockerSocket, id, imageName, true); err != nil {
		return
	}

	if err := os.MkdirAll(id.HomePath(), 0700); err != nil {
		return err
	}

	user := data.User
	if user == "" {
		user = "container"
	}

	containerData := ContainerInitScript{
		data.User == "",
		user,
		data.Uid,
		data.Gid,
		strings.Join(data.Command, " "),
		len(data.Volumes) > 0,
		strings.Join(data.Volumes, " "),
		data.Ports,
		data.SocketActivated,
	}

	file, _, err := utils.OpenFileExclusive(path.Join(id.HomePath(), "container-init.sh"), 0700)
	if err != nil {
		fmt.Printf("container init pre-start: Unable to open script file: %v\n", err)
		return err
	}
	defer file.Close()

	if erre := ContainerInitTemplate.Execute(file, containerData); erre != nil {
		fmt.Printf("container init pre-start: Unable to output template: ", erre)
		return erre
	}
	if err := file.Close(); err != nil {
		return err
	}

	file, _, err = utils.OpenFileExclusive(path.Join(id.HomePath(), "container-cmd.sh"), 0705)
	if err != nil {
		fmt.Printf("container init pre-start: Unable to open cmd script file: %v\n", err)
		return err
	}
	defer file.Close()

	if erre := ContainerCmdTemplate.Execute(file, containerData); erre != nil {
		fmt.Printf("container init pre-start: Unable to output cmd template: ", erre)
		return erre
	}
	if err := file.Close(); err != nil {
		return err
	}

	return nil
}

func createUser(id Identifier) error {
	cmd := exec.Command("/usr/sbin/useradd", id.LoginFor(), "-m", "-d", id.HomePath(), "-c", "Container user")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Println(out)
		return err
	}
	selinux.RestoreCon(id.HomePath(), true)
	return nil
}
