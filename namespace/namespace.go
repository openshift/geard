package namespace

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/kraman/libcontainer"
	"github.com/kraman/libcontainer/namespaces"
	"github.com/kraman/libcontainer/utils"
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
