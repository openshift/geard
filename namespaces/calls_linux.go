package namespaces

import (
	"syscall"
)

const (
	SYS_SETNS = 308 // look here for different arch http://git.kernel.org/cgit/linux/kernel/git/torvalds/linux.git/commit/?id=7b21fddd087678a70ad64afc0f632e0f1071b092
)

func chroot(dir string) error {
	return syscall.Chroot(dir)
}

func chdir(dir string) error {
	return syscall.Chdir(dir)
}

func exec(cmd string, args []string, env []string) error {
	return syscall.Exec(cmd, args, env)
}

func fork() (int, error) {
	syscall.ForkLock.Lock()
	pid, _, err := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	syscall.ForkLock.Unlock()
	if err != 0 {
		return -1, err
	}
	return int(pid), nil
}

func vfork() (int, error) {
	syscall.ForkLock.Lock()
	pid, _, err := syscall.Syscall(syscall.SYS_VFORK, 0, 0, 0)
	syscall.ForkLock.Unlock()
	if err != 0 {
		return -1, err
	}
	return int(pid), nil
}

func mount(source, target, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}

func unmount(target string, flags int) error {
	return syscall.Unmount(target, flags)
}

func pivotroot(newroot, putold string) error {
	return syscall.PivotRoot(newroot, putold)
}

func unshare(flags int) error {
	return syscall.Unshare(flags)
}

func clone(flags uintptr) (int, error) {
	syscall.ForkLock.Lock()
	pid, _, err := syscall.RawSyscall(syscall.SYS_CLONE, flags, 0, 0)
	syscall.ForkLock.Unlock()
	if err != 0 {
		return -1, err
	}
	return int(pid), nil
}

func setns(fd uintptr, flags uintptr) error {
	_, _, err := syscall.RawSyscall(SYS_SETNS, fd, flags, 0)
	if err != 0 {
		return err
	}
	return nil
}

func usetCloseOnExec(fd uintptr) error {
	if _, _, err := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFD, 0); err != 0 {
		return err
	}
	return nil
}

func setgroups(gids []int) error {
	return syscall.Setgroups(gids)
}

func setresgid(rgid, egid, sgid int) error {
	return syscall.Setresgid(rgid, egid, sgid)
}

func setresuid(ruid, euid, suid int) error {
	return syscall.Setresuid(ruid, euid, suid)
}

func sethostname(name string) error {
	if len(name) > 12 {
		name = name[:12]
	}
	return syscall.Sethostname([]byte(name))
}

func setsid() (int, error) {
	return syscall.Setsid()
}
