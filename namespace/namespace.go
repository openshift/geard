package namespace

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/kraman/libcontainer"
	"github.com/kraman/libcontainer/namespaces"
	"github.com/kraman/libcontainer/utils"
	"github.com/openshift/geard/docker"
)

func createContainer(containerName string, nsPid int, args []string, env []string) (*libcontainer.Container, error) {
	container := new(libcontainer.Container)
	container.ID = containerName
	container.NsPid = nsPid
	container.Command = &libcontainer.Command{args, env}
	container.Namespaces = []libcontainer.Namespace{
		libcontainer.CLONE_NEWNS,
		libcontainer.CLONE_NEWUTS,
		libcontainer.CLONE_NEWIPC,
		libcontainer.CLONE_NEWPID,
		libcontainer.CLONE_NEWNET,
	}
	container.Capabilities = []libcontainer.Capability{
		libcontainer.CAP_SETPCAP,
		libcontainer.CAP_SYS_MODULE,
		libcontainer.CAP_SYS_RAWIO,
		libcontainer.CAP_SYS_PACCT,
		libcontainer.CAP_SYS_ADMIN,
		libcontainer.CAP_SYS_NICE,
		libcontainer.CAP_SYS_RESOURCE,
		libcontainer.CAP_SYS_TIME,
		libcontainer.CAP_SYS_TTY_CONFIG,
		libcontainer.CAP_MKNOD,
		libcontainer.CAP_AUDIT_WRITE,
		libcontainer.CAP_AUDIT_CONTROL,
		libcontainer.CAP_MAC_OVERRIDE,
		libcontainer.CAP_MAC_ADMIN,
	}
	netns_path := path.Join("/proc", strconv.Itoa(nsPid), "ns", "net")
	f, err := os.Open(netns_path)
	if err != nil {
		return nil, err
	}
	container.NetNsFd = f.Fd()

	return container, nil
}

func RunIn(containerName string, nsPid int, args []string, env []string) (int, error) {
	container, err := createContainer(containerName, nsPid, args, env)
	if err != nil {
		return -1, fmt.Errorf("error creating container %s", err)
	}

	pid, err := namespaces.ExecIn(container, container.Command)
	if err != nil {
		return -1, fmt.Errorf("error execin container %s", err)
	}
	exitcode, err := utils.WaitOnPid(pid)
	if err != nil {
		return -1, fmt.Errorf("error waiting on child %s", err)
	}
	return exitcode, nil
}

func RunCommandInContainer(client *docker.DockerClient, name string, command []string, environment []string) (int, error) {
	if len(command) == 0 {
		fmt.Println("No command specified")
		os.Exit(3)
	}

	container, err := client.InspectContainer(name)
	if err != nil {
		fmt.Printf("Unable to locate container named %v\n", name)
		os.Exit(3)
	}
	containerNsPID, err := client.ChildProcessForContainer(container)
	if err != nil {
		fmt.Println("Couldn't create child process for container")
		os.Exit(3)
	}

	containerEnv := environment

	if len(containerEnv) == 0 {
		containerEnv = container.Config.Env
	}

	return RunIn(name, containerNsPID, command, containerEnv)
}
