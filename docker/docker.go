package docker

import (
	"errors"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/fsouza/go-dockerclient/engine"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type containerLookupResult struct {
	Container *docker.Container
	Error     error
}

type DockerClient struct {
	client          *docker.Client
	executionDriver string
}

func (d *DockerClient) ListContainers() ([]docker.APIContainers, error) {
	return d.client.ListContainers(docker.ListContainersOptions{All: true})
}

func (d *DockerClient) ForceCleanContainer(ID string) error {
	if err := d.client.KillContainer(ID); err != nil {
		return err
	}
	return d.client.RemoveContainer(docker.RemoveContainerOptions{ID, true})
}

func lookupContainer(containerName string, client *docker.Client, waitForContainer bool) containerLookupResult {
	timeout := 0
	if waitForContainer {
		timeout = 60
	}

	for i := 0; i <= timeout; i++ {
		if container, err := client.InspectContainer(containerName); err != nil {
			if !strings.HasPrefix(err.Error(), "No such container") {
				return containerLookupResult{nil, err}
			}
			if timeout > 0 {
				time.Sleep(time.Second)
			}
		} else {
			return containerLookupResult{container, nil}
		}
	}
	return containerLookupResult{nil, fmt.Errorf("Container not active")}
}

func GetConnection(dockerSocket string) (*DockerClient, error) {
	var (
		client          *docker.Client
		err             error
		info            *engine.Env
		executionDriver string
	)

	client, err = docker.NewClient(dockerSocket)
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

func (d *DockerClient) GetContainer(containerName string, waitForContainer bool) (*docker.Container, error) {
	timeoutChannel := make(chan containerLookupResult, 1)
	var container *docker.Container
	go func() { timeoutChannel <- lookupContainer(containerName, d.client, waitForContainer) }()
	select {
	case cInfo := <-timeoutChannel:
		if cInfo.Error != nil {
			return nil, cInfo.Error
		}
		container = cInfo.Container
	case <-time.After(time.Minute):
		return nil, fmt.Errorf("Timeout waiting for container")
	}

	return container, nil
}

func (d *DockerClient) GetImage(imageName string) (*docker.Image, error) {
	if img, err := d.client.InspectImage(imageName); err != nil {
		if err == docker.ErrNoSuchImage {
			if err := d.client.PullImage(docker.PullImageOptions{imageName, "", os.Stdout}, docker.AuthConfiguration{}); err != nil {
				return nil, err
			}
			return d.client.InspectImage(imageName)
		}
		return nil, err
	} else {
		return img, err
	}
}

func (d *DockerClient) ChildProcessForContainer(container *docker.Container) (int, error) {
	log.Printf("docker: execution driver %s", d.executionDriver)
	if d.executionDriver == "" || strings.HasPrefix(d.executionDriver, "lxc") {
		//Parent pid (LXC or N-Spawn)
		ppid := strconv.Itoa(container.State.Pid)
		log.Printf("docker: parent pid %s", ppid)

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
		return 0, fmt.Errorf("Container not found")
	}
	return 0, errors.New(fmt.Sprintf("Unable to find child process for container", container.ID))
}
