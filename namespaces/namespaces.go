/*
   TODO
   pivot root
   cgroups
   more mount stuff that I probably am forgetting
   apparmor
*/

package namespaces

import (
	"github.com/crosbymichael/libcontainer"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// getNsFds inspects the container's namespace configuration and opens the fds to
// each of the namespaces.
func getNsFds(container *libcontainer.Container) ([]uintptr, error) {
	var (
		namespaces = []string{}
		fds        = []uintptr{}
	)

	for _, ns := range container.Namespaces {
		namespaces = append(namespaces, namespaceFileMap[ns])
	}

	for _, ns := range namespaces {
		fd, err := getNsFd(container.NsPid, ns)
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

// getNsFd returns the fd for a specific pid and namespace option
func getNsFd(pid int, ns string) (uintptr, error) {
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
func getNamespaceFlags(container *libcontainer.Container) (flag int) {
	for _, ns := range container.Namespaces {
		flag |= namespaceMap[ns]
	}
	return
}

// setupEnvironment adds additional environment variables to the container's
// Command such as USER, LOGNAME, container, and TERM
func setupEnvironment(container *libcontainer.Container) {
	addEnvIfNotSet(container, "container", "docker")
	// TODO: check if pty
	addEnvIfNotSet(container, "TERM", "xterm")
	// TODO: get username from container
	addEnvIfNotSet(container, "USER", "root")
	addEnvIfNotSet(container, "LOGNAME", "root")
}

func setupUser(container *libcontainer.Container) error {
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

func getMasterAndConsole(container *libcontainer.Container) (string, *os.File, error) {
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
func joinExistingNetworkNamespace(container *libcontainer.Container, needsUnshare bool) error {
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
