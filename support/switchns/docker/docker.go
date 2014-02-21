package docker

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func MapContainerName(container_name string) (string, error) {
	endpoint := "unix:///var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		fmt.Println("Unable to connect to docker server:", err.Error())
		return "", err
	}
	container, err := client.InspectContainer(container_name)
	if err != nil {
		fmt.Println("Unable to inspect container "+container_name, err.Error())
		return "", err
	}

	//Parent pid (LXC or N-Spawn)
	ppid := strconv.Itoa(container.State.Pid)

	//Lookup any child of parent pid
	pstatFiles, _ := filepath.Glob(path.Join("/proc", "*", "stat"))
	for _, pstatFile := range pstatFiles {
		pstatBytes, err := ioutil.ReadFile(pstatFile)
		if err != nil {
			continue
		}
		pstatArr := strings.Split(string(pstatBytes), " ")
		if pstatArr[3] == ppid {
			pid := strings.Split(pstatFile, "/")[2]
			return pid, nil
		}
	}

	return "", fmt.Errorf("Unable to find child process for container %v", container_name)
}
