package docker

import (
	"errors"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
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
			fmt.Printf("waiting for container... %v\n", i)
			if timeout > 0 {
				time.Sleep(time.Second)
			}
		} else {
			return containerLookupResult{container, nil}
		}
	}
	return containerLookupResult{nil, fmt.Errorf("Container not active")}
}

func GetConnection(dockerSocket string) (*docker.Client, error) {
	client, err := docker.NewClient(dockerSocket)
	if err != nil {
		fmt.Println("Unable to connect to docker server:", err.Error())
		return nil, err
	}
	return client, nil
}

func GetContainer(dockerSocket string, containerName string, waitForContainer bool) (*docker.Client, *docker.Container, error) {
	client, err := GetConnection(dockerSocket)
	if err != nil {
		return nil, nil, err
	}

	timeoutChannel := make(chan containerLookupResult, 1)
	var container *docker.Container
	go func() { timeoutChannel <- lookupContainer(containerName, client, waitForContainer) }()
	select {
	case cInfo := <-timeoutChannel:
		if cInfo.Error != nil {
			return nil, nil, cInfo.Error
		}
		container = cInfo.Container
	case <-time.After(time.Minute):
		return nil, nil, fmt.Errorf("Timeout waiting for container")
	}

	return client, container, nil
}

func GetImage(dockerSocket string, imageName string) (*docker.Image, error) {
	client, err := GetConnection(dockerSocket)
	if err != nil {
		return nil, err
	}

	if img, err := client.InspectImage(imageName); err != nil {
		if err == docker.ErrNoSuchImage {
			if err := client.PullImage(docker.PullImageOptions{imageName, "", os.Stdout}, docker.AuthConfiguration{}); err != nil {
				return nil, err
			}
			return client.InspectImage(imageName)
		}
		return nil, err
	} else {
		return img, err
	}
}

func ChildProcessForContainer(container *docker.Container) (int, error) {
	//Parent pid (LXC or N-Spawn)
	ppid := strconv.Itoa(container.State.Pid)

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

	return 0, errors.New(fmt.Sprintf("Unable to find child process for container", container.ID))
}
