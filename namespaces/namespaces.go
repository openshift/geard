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
	"github.com/kraman/libcontainer"
	"github.com/kraman/libcontainer/utils"
	"os"
	"path/filepath"
	"syscall"
)

type Action func() error

func CloneIntoNamespace(namespaces libcontainer.Namespaces, action Action) (int, error) {
	// we need CLONE_VFORK so we can wait on the child
	flag := getNamespaceFlags(namespaces) | CLONE_VFORK

	pid, err := clone(uintptr(flag | SIGCHLD))
	if err != nil {
		return -1, fmt.Errorf("error cloning process: %s", err)
	}

	if pid == 0 {
		// welcome to your new namespace ;)
		if err := action(); err != nil {
			writeError("action %s", err)
		}
		os.Exit(0)
	}
	return pid, err
}

// CreateNewNamespace creates a new namespace and binds it's fd to the specified path
func CreateNewNamespace(namespace libcontainer.Namespace, bindTo string) error {
	var (
		flag   = namespaceMap[namespace]
		name   = namespaceFileMap[namespace]
		nspath = filepath.Join("/proc/self/ns", name)
	)
	// TODO: perform validation on name and flag

	pid, err := fork()
	if err != nil {
		return err
	}

	if pid == 0 {
		if err := unshare(flag); err != nil {
			writeError("unshare %s", err)
		}
		if err := mount(nspath, bindTo, "none", syscall.MS_BIND, ""); err != nil {
			writeError("bind mount %s", err)
		}
		os.Exit(0)
	}
	exit, err := utils.WaitOnPid(pid)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("exit status %d", exit)
	}
	return err
}

// RunInNamespace executes the action in the namespace
// specified by the fd passed
func RunInNamespace(fd uintptr, action Action) error {
	pid, err := fork()
	if err != nil {
		return err
	}
	if pid == 0 {
		if err := setns(fd, 0); err != nil {
			writeError("setns %s", err)
		}
		if err := action(); err != nil {
			writeError("action %s", err)
		}
		os.Exit(0)
	}
	exit, err := utils.WaitOnPid(pid)
	if err != nil {
		return err
	}
	if exit != 0 {
		fmt.Errorf("exit status %d", exit)
	}
	return nil
}

func JoinExistingNamespace(fd uintptr, ns libcontainer.Namespace) error {
	flag := namespaceMap[ns]
	if err := setns(fd, uintptr(flag)); err != nil {
		return err
	}
	return nil
}

// getNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func getNamespaceFlags(namespaces libcontainer.Namespaces) (flag int) {
	for _, ns := range namespaces {
		flag |= namespaceMap[ns]
	}
	return
}
