/*
   TODO
   pivot root
   cgroups
   more mount stuff that I probably am forgetting
   apparmor
*/

package namespaces

import (
	"errors"
	"fmt"
	"github.com/crosbymichael/libcontainer"
	"github.com/crosbymichael/libcontainer/capabilities"
	"github.com/crosbymichael/libcontainer/utils"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

var (
	// default mount point options
	defaults = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
)

var (
	ErrExistingNetworkNamespace = errors.New("specified both CLONE_NEWNET and an existing network namespace")
)

// New creates a new linux namespace backend for executing
// processes inside a container
func New() libcontainer.Backend {
	return &backend{}
}

type backend struct {
}

// Exec will spawn new namespaces with the specified Container configuration
// in the RootFs path and return the pid of the new containerized process.
//
// If an existing network namespace is specified the container
// will join that namespace.  If an existing network namespace is not specified but CLONE_NEWNET is,
// the container will be spawned with a new network namespace with no configuration.  Omiting an
// existing network namespace and the CLONE_NEWNET option in the container configuration will allow
// the container to the the host's networking options and configuration.
func (b *backend) Exec(container *libcontainer.Container) (int, error) {
	if container.NetworkNamespace != "" && container.Namespaces.Contains(libcontainer.CLONE_NEWNET) {
		return -1, ErrExistingNetworkNamespace
	}
	rootfs, err := filepath.Abs(container.RootFs)
	if err != nil {
		return -1, err
	}

	// we need CLONE_VFORK so we can wait on the child
	flag := b.getNamespaceFlags(container) | CLONE_VFORK

	pid, err := clone(uintptr(flag | SIGCHLD))
	if err != nil {
		return -1, fmt.Errorf("error cloning process: %s", err)
	}

	if pid == 0 {
		// welcome to your new namespace ;)
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

		// the network namespace must be joined before chrooting the process
		if container.NetworkNamespace != "" {
			if err := b.joinExistingNetworkNamespace(container, true); err != nil {
				writeError("join existing network namespace %s", err)
			}
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

		if err := capabilities.DropCapabilities(container); err != nil {
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

	container.NsPid = pid

	return pid, nil
}

// ExecIn will spawn a new command inside an existing container's namespaces.  The existing container's
// pid and namespace configuration is needed along with the specific capabilities that should
// be dropped once inside the namespace.
func (b *backend) ExecIn(container *libcontainer.Container, cmd *libcontainer.Command) (int, error) {
	if container.NsPid <= 0 {
		return -1, libcontainer.ErrInvalidPid
	}

	closeFds := func(fds []uintptr) {
		for _, fd := range fds {
			syscall.Close(int(fd))
		}
	}

	fds, err := b.getNsFds(container)
	if err != nil {
		return -1, err
	}

	pid, err := fork()
	if err != nil {
		closeFds(fds)
		return -1, err
	}

	if pid == 0 {
		if container.NetworkNamespace != "" {
			if err := b.joinExistingNetworkNamespace(container, false); err != nil {
				writeError("join netns %s %s", container.NetworkNamespace, err)
			}
		}

		for _, fd := range fds {
			if fd > 0 {
				if err := setns(fd, 0); err != nil {
					closeFds(fds)
					writeError("setns %s", err)
				}
			}
			syscall.Close(int(fd))
		}

		// important:
		// we need to fork and unshare so that re can remount proc and sys within
		// the namespace so the CLONE_NEWPID namespace will take effect
		// if we don't fork we would end up unmounting proc and sys for the entire
		// namespace
		child, err := fork()
		if err != nil {
			writeError("fork child %s", err)
		}

		if child == 0 {
			if err := unshare(CLONE_NEWNS); err != nil {
				writeError("unshare newns %s", err)
			}
			if err := b.remountProc(); err != nil {
				writeError("remount proc %s", err)
			}
			if err := b.remountSys(); err != nil {
				writeError("remount sys %s", err)
			}
			if err := capabilities.DropCapabilities(container); err != nil {
				writeError("drop caps %s", err)
			}

			err = exec(cmd.Args[0], cmd.Args[0:], cmd.Env)
			panic(err)
		}
		utils.WaitOnPid(child)
		os.Exit(0)
	}
	return pid, err
}

// getNsFds inspects the container's namespace configuration and opens the fds to
// each of the namespaces.
func (b *backend) getNsFds(container *libcontainer.Container) ([]uintptr, error) {
	var (
		namespaces = []string{}
		fds        = []uintptr{}
	)

	for _, ns := range container.Namespaces {
		namespaces = append(namespaces, namespaceFileMap[ns])
	}

	for _, ns := range namespaces {
		fd, err := b.getNsFd(container.NsPid, ns)
		if err != nil {
			for _, fd = range fds {
				syscall.Close(int(fd))
			}
			return nil, err
		}
		fds = append(fds, fd)
	}
	return fds, nil
}

func (b *backend) remountProc() error {
	if err := unmount("/proc", syscall.MNT_DETACH); err != nil {
		return err
	}
	if err := mount("proc", "/proc", "proc", uintptr(defaults), ""); err != nil {
		return err
	}
	return nil
}

func (b *backend) remountSys() error {
	if err := unmount("/sys", syscall.MNT_DETACH); err != nil {
		if err != syscall.EINVAL {
			return err
		}
	} else {
		if err := mount("sysfs", "/sys", "sysfs", uintptr(defaults), ""); err != nil {
			return err
		}
	}
	return nil
}

// mountSystem sets up linux specific system mounts like sys, proc, shm, and devpts
// inside the mount namespace
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

// getNsFd returns the fd for a specific pid and namespace option
func (b *backend) getNsFd(pid int, ns string) (uintptr, error) {
	nspath := filepath.Join("/proc", strconv.Itoa(pid), "ns", ns)
	// OpenFile adds closOnExec
	f, err := os.OpenFile(nspath, os.O_RDONLY, 0666)
	if err != nil {
		return 0, err
	}
	return f.Fd(), nil
}

// getNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func (b *backend) getNamespaceFlags(container *libcontainer.Container) (flag int) {
	for _, ns := range container.Namespaces {
		flag |= namespaceMap[ns]
	}
	return
}

// setupEnvironment adds additional environment variables to the container's
// Command such as USER, LOGNAME, container, and TERM
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

// joinExistingNetworkNamespace uses the NetworkNamespace file path defined on the container
// and joins the existing net namespace.
func (b *backend) joinExistingNetworkNamespace(container *libcontainer.Container, needsUnshare bool) error {
	f, err := os.Open(container.NetworkNamespace)
	if err != nil {
		return err
	}
	defer f.Close()

	if needsUnshare {
		// leave our parent's networking namespace
		if err := unshare(CLONE_NEWNET); err != nil {
			return err
		}
	}

	// join the new namespace specified by the fd
	if err := setns(f.Fd(), CLONE_NEWNET); err != nil {
		return err
	}
	return nil
}
