package namespaces

import (
	"fmt"
	"github.com/crosbymichael/libcontainer"
	"os"
	goexec "os/exec"
	"strconv"
	"syscall"
)

func SetupNamespaceMountDir(root string) error {
	if err := os.MkdirAll(root, 0666); err != nil {
		return err
	}

	// make sure mounts are not unmounted by other mnt namespaces
	if err := mount("", root, "none", syscall.MS_SHARED|syscall.MS_REC, ""); err != nil {
		return err
	}
	if err := mount(root, root, "none", syscall.MS_BIND, ""); err != nil {
		return err
	}
	return nil
}

// creates a new network namespace and binds it to the fd
// at the binding path
func CreateNetworkNamespace(bindingPath string) error {
	f, err := os.OpenFile(bindingPath, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0)
	if err != nil {
		return err
	}
	f.Close()

	if err := unshare(CLONE_NEWNET); err != nil {
		return err
	}

	if err := mount("/proc/self/ns/net", bindingPath, "none", syscall.MS_BIND, ""); err != nil {
		return err
	}
	return nil
}

func initializeNetwork() {

}

// ip link add veth0 type veth peer name veth1

// wait for ns/net to exist
// ip link set veth1 netns pid
// ip link set veth0 up
// ip link set veth0 master docker0
// ip netns exec NAME ip link set veth1 name eth0

// ip netns exec NAME ip link set eth0 up

func setupNetworking(container *libcontainer.Container, pid int) error {
	setup := [][]string{
		{"link", "add", "veth0", "type", "veth", "peer", "name", "veth1"},
		{"link", "set", "veth1", "netns", strconv.Itoa(pid)},
		{"link", "set", "veth0", "up"},
		{"link", "set", "veth0", "master", "docker0"},
	}
	for _, call := range setup {
		if _, err := ipcall(call...); err != nil {
			return fmt.Errorf("error calling ip %v: %s", call, err)
		}
	}
	return nil
}

func ipcall(args ...string) (string, error) {
	output, err := goexec.Command("/bin/ip", args...).Output()
	if err != nil {
		return "", fmt.Errorf("%s error running ip %v %s", err, args, string(output))
	}
	return string(output), nil
}
