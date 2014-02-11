package namespaces

import (
	"github.com/crosbymichael/libcontainer"
	"github.com/crosbymichael/libcontainer/network"
	"os"
	"syscall"
)

func SetupNamespaceMountDir(root string) error {
	if err := os.MkdirAll(root, 0666); err != nil {
		return err
	}
	// make sure mounts are not unmounted by other mnt namespaces
	if err := mount("", root, "none", syscall.MS_SHARED|syscall.MS_REC, ""); err != nil && err != syscall.EINVAL {
		return err
	}
	if err := mount(root, root, "none", syscall.MS_BIND, ""); err != nil {
		return err
	}
	return nil
}

// creates a new network namespace and binds it to the fd
// at the binding path
func CreateNetworkNamespace(bindingPath string) (int, error) {
	f, err := os.OpenFile(bindingPath, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0)
	if err != nil {
		return -1, err
	}
	f.Close()

	pid, err := fork()
	if err != nil {
		return -1, err
	}
	if pid == 0 {
		if err := unshare(CLONE_NEWNET); err != nil {
			writeError("unshare netns %s", err)
		}
		if err := mount("/proc/self/ns/net", bindingPath, "none", syscall.MS_BIND, ""); err != nil {
			writeError("bind mount netns %s", err)
		}
		os.Exit(0)
	}
	return pid, nil
}

func SetupNetworkNamespace(fd uintptr, config *libcontainer.Network) (int, error) {
	pid, err := fork()
	if err != nil {
		return -1, err
	}

	if pid == 0 {
		if err := setns(fd, CLONE_NEWNET); err != nil {
			writeError("unable to setns %s", err)
		}
		if err := network.InterfaceDown(config.TempVethName); err != nil {
			writeError("interface down %s %s", config.TempVethName, err)
		}
		if err := network.ChangeInterfaceName(config.TempVethName, "eth0"); err != nil {
			writeError("change %s to eth0 %s", config.TempVethName, err)
		}
		if network.SetInterfaceIp("eth0", config.IP); err != nil {
			writeError("set eth0 ip %s", err)
		}
		// TODO: set mtu
		if network.InterfaceUp("eth0"); err != nil {
			writeError("eth0 up %s", err)
		}
		if network.InterfaceUp("lo"); err != nil {
			writeError("lo up %s", err)
		}

		if config.Gateway != "" {
			if network.SetDefaultGateway(config.Gateway); err != nil {
				writeError("set gateway to %s %s", config.Gateway, err)
			}
		}
		os.Exit(0)
	}
	return pid, nil
}

func DeleteNetworkNamespace(bindingPath string) error {
	if err := unmount(bindingPath, 0); err != nil {
		return err
	}
	return os.Remove(bindingPath)
}
