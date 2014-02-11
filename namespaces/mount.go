package namespaces

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var (
	// default mount point options
	defaults = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
)

func SetupNewMountNamespace(rootfs string, readonly bool) error {
	if err := mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mounting / as slave %s", err)
	}

	if err := mount(rootfs, rootfs, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mouting %s as bind %s", rootfs, err)
	}

	if readonly {
		if err := mount(rootfs, rootfs, "bind", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, ""); err != nil {
			return fmt.Errorf("mounting %s as readonly %s", rootfs, err)
		}
	}

	if err := mountSystem(rootfs); err != nil {
		return fmt.Errorf("mount system %s", err)
	}

	if err := chdir(rootfs); err != nil {
		return fmt.Errorf("chdir into %s %s", rootfs, err)
	}

	if err := mount(rootfs, "/", "", syscall.MS_MOVE, ""); err != nil {
		return fmt.Errorf("mount move %s into / %s", rootfs, err)
	}
	return nil
}

// mountSystem sets up linux specific system mounts like sys, proc, shm, and devpts
// inside the mount namespace
func mountSystem(rootfs string) error {
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

func remountProc() error {
	if err := unmount("/proc", syscall.MNT_DETACH); err != nil {
		return err
	}
	if err := mount("proc", "/proc", "proc", uintptr(defaults), ""); err != nil {
		return err
	}
	return nil
}

func remountSys() error {
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
