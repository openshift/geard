/*
   TODO
   pivot root
   cgroups
   more mount stuff that I probably am forgetting
   apparmor
*/

package namespaces

import (
	"fmt"
	"github.com/crosbymichael/libcontainer"
	"github.com/crosbymichael/libcontainer/network"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

var (
	defaults = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
)

func New() libcontainer.Backend {
	return &backend{}
}

type backend struct {
}

func (b *backend) Exec(container *libcontainer.Container) (int, error) {
	rootfs, err := filepath.Abs(container.RootFs)
	if err != nil {
		return -1, err
	}
	mtu, err := network.GetDefaultMtu()
	if err != nil {
		return -1, err
	}

	// we need CLONE_VFORK so we can wait on the child
	flag := b.getNamespaceFlags(container) | CLONE_VFORK

	// setup pipes to sync parent and child
	childR, parentW, err := os.Pipe()
	if err != nil {
		return -1, err
	}
	usetCloseOnExec(childR.Fd())

	pid, err := clone(uintptr(flag | SIGCHLD))
	if err != nil {
		return -1, fmt.Errorf("error cloning process: %s", err)
	}

	if pid == 0 {
		// welcome to your new namespace ;)
		parentW.Close()

		if _, err := setsid(); err != nil {
			writeError("setsid %s", err)
		}

		if err := mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
			writeError("mounting / as slave", err)
		}

		if err := mount(rootfs, rootfs, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
			writeError("mouting %s as bind %s", rootfs, err)
		}

		if container.ReadonlyFs {
			if err := mount(rootfs, rootfs, "bind", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, ""); err != nil {
				writeError("mounting %s as readonly %s", rootfs, err)
			}
		}

		if err := b.mountSystem(rootfs); err != nil {
			writeError("mount system %s", err)
		}

		if err := chdir(rootfs); err != nil {
			writeError("chdir into %s %s", rootfs, err)
		}

		if err := mount(rootfs, "/", "", syscall.MS_MOVE, ""); err != nil {
			writeError("mount move %s into / %s", rootfs, err)
		}

		if err := chroot("."); err != nil {
			writeError("chroot . %s", err)
		}

		if err := chdir("/"); err != nil {
			writeError("chdir / %s", err)
		}

		if err := sethostname(container.ID); err != nil {
			writeError("sethostname %s", err)
		}

		if err := b.setupNetwork(container, mtu); err != nil {
			writeError("setup networking %s", err)
		}

		if err := libcontainer.DropCapabilities(container); err != nil {
			writeError("drop capabilities %s", err)
		}

		if err := b.setupUser(container); err != nil {
			writeError("setup user %s", err)
		}

		if container.WorkingDir != "" {
			if err := chdir(container.WorkingDir); err != nil {
				writeError("chdir to %s %s", container.WorkingDir, err)
			}
		}

		err = exec(container.Command.Args[0], container.Command.Args[0:], container.Command.Env)
		// only reachable if exec errors
		panic(err)
	}

	childR.Close()
	container.NsPid = pid

	return pid, nil
}

func (b *backend) mountSystem(rootfs string) error {
	mounts := []struct {
		source string
		path   string
		device string
		flags  int
		data   string
	}{
		{source: "proc", path: filepath.Join(rootfs, "proc"), device: "proc", flags: defaults},
		{source: "sysfs", path: filepath.Join(rootfs, "sys"), device: "sysfs", flags: defaults},
		{source: "shm", path: filepath.Join(rootfs, "dev", "shm"), device: "tmpfs", flags: defaults, data: "size=65536k"},
		{source: "devpts", path: filepath.Join(rootfs, "dev", "pts"), device: "devpts", flags: syscall.MS_NOSUID | syscall.MS_NOEXEC, data: "ptmxmode=0666"},
	}

	for _, m := range mounts {
		if err := os.MkdirAll(m.path, 0666); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdirall %s %s", m.path, err)
		}
		if err := mount(m.source, m.path, m.device, uintptr(m.flags), m.data); err != nil {
			return fmt.Errorf("mounting %s into %s %s", m.source, m.path, err)
		}
	}
	return nil
}

func (b *backend) ExecIn(container *libcontainer.Container, cmd *libcontainer.Command) (int, error) {
	if container.NsPid <= 0 {
		return -1, libcontainer.ErrInvalidPid
	}

	var (
		namespaces = []string{}
		fds        = []uintptr{}
		closeFds   = func(fds []uintptr) {
			for _, fd := range fds {
				syscall.Close(int(fd))
			}
		}
	)

	for _, ns := range container.Namespaces {
		namespaces = append(namespaces, namespaceFileMap[ns])
	}

	for _, ns := range namespaces {
		fd, err := b.getFd(container.NsPid, ns)
		if err != nil {
			closeFds(fds)
			return -1, err
		}
		fds = append(fds, fd)
	}

	pid, err := fork()
	if err != nil {
		closeFds(fds)
		return -1, err
	}

	if pid == 0 {
		for _, fd := range fds {
			if fd > 0 {
				if err := setns(fd, 0); err != nil {
					closeFds(fds)
					writeError("setns %s", err)
				}
			}
			syscall.Close(int(fd))
		}

		child, err := fork()
		if err != nil {
			writeError("fork child %s", err)
		}

		if child == 0 {
			if err := unshare(CLONE_NEWNS); err != nil {
				writeError("unshare newns %s", err)
			}

			if err := unmount("/proc", syscall.MNT_DETACH); err != nil {
				writeError("unmount /proc %s", err)
			}

			if err := mount("proc", "/proc", "proc", uintptr(defaults), ""); err != nil {
				writeError("mount proc %s", err)
			}

			if err := unmount("/sys", syscall.MNT_DETACH); err != nil {
				if err != syscall.EINVAL {
					writeError("umount /sys %s", err)
				}
			} else {
				if err := mount("sysfs", "/sys", "sysfs", uintptr(defaults), ""); err != nil {
					writeError("mount /sys %s", err)
				}
			}

			if err := libcontainer.DropCapabilities(container); err != nil {
				writeError("drop caps %s", err)
			}

			err = exec(cmd.Args[0], cmd.Args[0:], cmd.Env)
			panic(err)
		}
		libcontainer.WaitOnPid(child)
	}
	return pid, err
}

func (b *backend) getFd(pid int, ns string) (uintptr, error) {
	nspath := filepath.Join("/proc", strconv.Itoa(pid), "ns", ns)
	// OpenFile adds closOnExec
	f, err := os.OpenFile(nspath, os.O_RDONLY, 0666)
	if err != nil {
		return 0, err
	}
	return f.Fd(), nil
}

func (b *backend) getNamespaceFlags(container *libcontainer.Container) (flag int) {
	for _, ns := range container.Namespaces {
		flag |= namespaceMap[ns]
	}
	return
}

func (b *backend) setupEnvironment(container *libcontainer.Container) {
	addEnvIfNotSet(container, "container", "docker")
	// TODO: check if pty
	addEnvIfNotSet(container, "TERM", "xterm")
	// TODO: get username from container
	addEnvIfNotSet(container, "USER", "root")
	addEnvIfNotSet(container, "LOGNAME", "root")
}

func (b *backend) setupUser(container *libcontainer.Container) error {
	// TODO: honor user passed on container
	if err := setgroups(nil); err != nil {
		return err
	}
	if err := setresgid(0, 0, 0); err != nil {
		return err
	}
	if err := setresuid(0, 0, 0); err != nil {
		return err
	}
	return nil
}

func (b *backend) getMasterAndConsole(container *libcontainer.Container) (string, *os.File, error) {
	master, err := openpmtx()
	if err != nil {
		return "", nil, err
	}

	console, err := ptsname(master)
	if err != nil {
		master.Close()
		return "", nil, err
	}
	return console, master, nil
}

func (b *backend) setupNetwork(container *libcontainer.Container, mtu int) error {
	// do not setup networking if the NEWNET namespace is not provided
	if !container.Namespaces.Contains(libcontainer.CLONE_NEWNET) {
		return nil
	}

	// TODO: get mtu from container
	if err := network.SetMtu("lo", mtu); err != nil {
		return err
	}
	if err := network.InterfaceUp("lo"); err != nil {
		return err
	}
	return nil
}
