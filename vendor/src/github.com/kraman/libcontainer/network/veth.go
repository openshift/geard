package network

import (
	"fmt"
	"github.com/kraman/libcontainer"
	"github.com/kraman/libcontainer/namespaces"
	"os"
	"syscall"
)

// SetupNetworkNamespace sets up an existing network namespace with the specified
// network configuration.
func SetupVethInsideNamespace(fd uintptr, config *libcontainer.Network) error {
	if err := namespaces.RunInNamespace(fd, func() error { return setupVeth(config) }); err != nil {
		return err
	}
	return nil
}

func setupVeth(config *libcontainer.Network) error {
	if err := InterfaceDown(config.TempVethName); err != nil {
		return fmt.Errorf("interface down %s %s", config.TempVethName, err)
	}
	if err := ChangeInterfaceName(config.TempVethName, "eth0"); err != nil {
		return fmt.Errorf("change %s to eth0 %s", config.TempVethName, err)
	}
	if err := SetInterfaceIp("eth0", config.IP); err != nil {
		return fmt.Errorf("set eth0 ip %s", err)
	}
	// TODO: set mtu
	if err := InterfaceUp("eth0"); err != nil {
		return fmt.Errorf("eth0 up %s", err)
	}
	if err := InterfaceUp("lo"); err != nil {
		return fmt.Errorf("lo up %s", err)
	}

	if config.Gateway != "" {
		if err := SetDefaultGateway(config.Gateway); err != nil {
			return fmt.Errorf("set gateway to %s %s", config.Gateway, err)
		}
	}
	return nil
}

// SetupNamespaceMountDir prepares a new root for use as a mount
// source for bind mounting namespace fd to an outside path
func SetupNamespaceMountDir(root string) error {
	if err := os.MkdirAll(root, 0666); err != nil {
		return err
	}
	// make sure mounts are not unmounted by other mnt namespaces
	if err := syscall.Mount("", root, "none", syscall.MS_SHARED|syscall.MS_REC, ""); err != nil && err != syscall.EINVAL {
		return err
	}
	if err := syscall.Mount(root, root, "none", syscall.MS_BIND, ""); err != nil {
		return err
	}
	return nil
}

// CreateNetworkNamespace creates a new network namespace and binds it's fd
// at the binding path
func CreateNetworkNamespace(bindingPath string) error {
	f, err := os.OpenFile(bindingPath, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0)
	if err != nil {
		return err
	}
	f.Close()

	if err := namespaces.CreateNewNamespace(libcontainer.CLONE_NEWNET, bindingPath); err != nil {
		return err
	}
	return nil
}

// DeleteNetworkNamespace unmounts the binding path and removes the
// file so that no references to the fd are present and the network
// namespace is automatically cleaned up
func DeleteNetworkNamespace(bindingPath string) error {
	if err := syscall.Unmount(bindingPath, 0); err != nil {
		return err
	}
	return os.Remove(bindingPath)
}
