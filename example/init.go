package main

import (
	"flag"
	"fmt"
	"github.com/dotcloud/docker/pkg/netlink"
	"github.com/syndtr/gocapability/capability"
	"io/ioutil"
	"net"
	"os"
	goexec "os/exec"
	"syscall"
)

func main() {
	fd := flag.Int("fd", -1, "synchronizing pipe")
	flag.Parse()
	if *fd == -1 {
		panic("no fd received")
	}

	waitForNetworking(*fd)
	if err := setupMounts(); err != nil {
		panic(err)
	}
	if err := initNetwork("172.17.0.100/16", "172.17.42.1"); err != nil {
		panic(err)
	}
	if err := dropCaps(); err != nil {
		panic(err)
	}

	err := syscall.Exec(flag.Arg(0), flag.Args()[0:], os.Environ())
	panic(err)
}

func waitForNetworking(fd int) {
	f := os.NewFile(uintptr(fd), "pipe")
	if _, err := ioutil.ReadAll(f); err != nil {
		panic(err)
	}
	f.Close()
}

func dropCaps() error {
	drop := []capability.Cap{
		capability.CAP_SETPCAP,
		capability.CAP_SYS_MODULE,
		capability.CAP_SYS_RAWIO,
		capability.CAP_SYS_PACCT,
		capability.CAP_SYS_ADMIN,
		capability.CAP_SYS_NICE,
		capability.CAP_SYS_RESOURCE,
		capability.CAP_SYS_TIME,
		capability.CAP_SYS_TTY_CONFIG,
		capability.CAP_MKNOD,
		capability.CAP_AUDIT_WRITE,
		capability.CAP_AUDIT_CONTROL,
		capability.CAP_MAC_OVERRIDE,
		capability.CAP_MAC_ADMIN,
	}

	c, err := capability.NewPid(os.Getpid())
	if err != nil {
		return err
	}

	c.Unset(capability.CAPS|capability.BOUNDS, drop...)

	if err := c.Apply(capability.CAPS | capability.BOUNDS); err != nil {
		return err
	}
	return nil
}

func setupMounts() error {
	defaultMountFlags := uintptr(syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC)

	if err := mount("proc", "proc", "proc", defaultMountFlags, ""); err != nil {
		return err
	}
	if err := mount("sysfs", "/sys", "sysfs", defaultMountFlags, ""); err != nil {
		return err
	}
	if err := mount("devpts", "/dev/pts", "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "newinstance,ptmxmode=0666"); err != nil {
		return err
	}
	if err := mount("shm", "/dev/shm", "tmpfs", defaultMountFlags, "size=65536k"); err != nil {
		return err
	}
	return nil
}

func initNetwork(sIp, gateway string) error {
	if _, err := ipcall("link", "set", "veth1", "name", "eth0"); err != nil {
		return err
	}

	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		return fmt.Errorf("Unable to set up networking: %v", err)
	}
	ip, ipNet, err := net.ParseCIDR(sIp)
	if err != nil {
		return fmt.Errorf("Unable to set up networking: %v", err)
	}
	if err := netlink.NetworkLinkAddIp(iface, ip, ipNet); err != nil {
		return fmt.Errorf("Unable to set up networking: %v", err)
	}
	if err := netlink.NetworkLinkUp(iface); err != nil {
		return fmt.Errorf("Unable to set up networking: %v", err)
	}

	// loopback
	iface, err = net.InterfaceByName("lo")
	if err != nil {
		return fmt.Errorf("Unable to set up networking: %v", err)
	}
	if err := netlink.NetworkLinkUp(iface); err != nil {
		return fmt.Errorf("Unable to set up networking: %v", err)
	}

	if gateway != "" {
		gw := net.ParseIP(gateway)
		if gw == nil {
			return fmt.Errorf("Unable to set up networking, %s is not a valid gateway IP", gateway)
		}
		if err := netlink.AddDefaultGw(gw); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
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

func mount(source, target, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}
