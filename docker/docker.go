package docker

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	gdocker "github.com/fsouza/go-dockerclient"
)

type containerLookupResult struct {
	Container *gdocker.Container
	Error     error
}

type DockerClient struct {
	client          *gdocker.Client
	executionDriver string
}

func (d *DockerClient) ListContainers() ([]gdocker.APIContainers, error) {
	return d.client.ListContainers(gdocker.ListContainersOptions{All: true})
}

func (d *DockerClient) ForceCleanContainer(ID string) error {
	if err := d.client.KillContainer(gdocker.KillContainerOptions{ID: ID}); err != nil {
		return err
	}
	return d.client.RemoveContainer(gdocker.RemoveContainerOptions{ID, true, true})
}

func GetConnection(dockerSocket string) (*DockerClient, error) {
	var (
		client          *gdocker.Client
		err             error
		info            *gdocker.Env
		executionDriver string
	)

	client, err = gdocker.NewClient(dockerSocket)
	if err != nil {
		fmt.Println("Unable to connect to docker server:", err.Error())
		return nil, err
	}

	if info, err = client.Info(); err != nil {
		return nil, err
	}
	executionDriver = info.Get("ExecutionDriver")

	return &DockerClient{client, executionDriver}, nil
}

var ErrNoSuchContainer = errors.New("can't find container")

func (d *DockerClient) InspectContainer(containerName string) (*gdocker.Container, error) {
	c, err := d.client.InspectContainer(containerName)
	if err != nil && strings.HasPrefix(err.Error(), "No such container") {
		err = ErrNoSuchContainer
	}
	return c, err
}

func (d *DockerClient) GetImage(imageName string) (*gdocker.Image, error) {
	if img, err := d.client.InspectImage(imageName); err != nil {
		if err == gdocker.ErrNoSuchImage {
			if err := d.client.PullImage(gdocker.PullImageOptions{imageName, "", "", os.Stdout, false}, gdocker.AuthConfiguration{}); err != nil {
				return nil, err
			}
			return d.client.InspectImage(imageName)
		}
		return nil, err
	} else {
		return img, err
	}
}

func (d *DockerClient) GetContainerIPs(ids []string) (map[string]string, error) {
	ips := make(map[string]string)
	for _, id := range ids {
		if cInfo, err := d.InspectContainer(id); err == nil {
			ips[cInfo.NetworkSettings.IPAddress] = id
		}
	}
	return ips, nil
}

func (d *DockerClient) ChildProcessForContainer(container *gdocker.Container) (int, error) {
	//log.Printf("docker: execution driver %s", d.executionDriver)
	if d.executionDriver == "" || strings.HasPrefix(d.executionDriver, "lxc") {
		//Parent pid (LXC or N-Spawn)
		ppid := strconv.Itoa(container.State.Pid)
		//log.Printf("docker: parent pid %s", ppid)

		//Lookup any child of parent pid
		files, _ := filepath.Glob(filepath.Join("/proc", "*", "stat"))
		for _, file := range files {
			bytes, err := ioutil.ReadFile(file)
			if err != nil {
				continue
			}
			pids := strings.Split(string(bytes), " ")
			if pids[3] == ppid {
				child := strings.Split(file, "/")[2]
				pid, err := strconv.Atoi(child)
				if err != nil {
					return 0, err
				}
				return pid, nil
			}
		}
	} else {
		if container.State.Pid != 0 {
			return container.State.Pid, nil
		}
		return 0, fmt.Errorf("unable to find child process for container %s - race condition with Docker?", container.ID)
	}
	return 0, fmt.Errorf("unable to find child process for container %s", container.ID)
}
