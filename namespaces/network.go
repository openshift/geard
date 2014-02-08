package namespaces

import (
	"fmt"
	"github.com/crosbymichael/libcontainer"
	goexec "os/exec"
	"strconv"
)

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
