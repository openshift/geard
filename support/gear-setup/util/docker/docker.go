package util

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
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

func GetContainer(containerName string, waitForContainer bool) (*docker.Client, *docker.Container, error) {
	endpoint := "unix:///var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		fmt.Println("Unable to connect to docker server:", err.Error())
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
