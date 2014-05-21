// +build linux

package utils

import (
	"errors"
	"os"
	"syscall"
)

var ErrLockTaken = errors.New("An exclusive lock already exists on the specified file.")

func OpenFileExclusive(path string, mode os.FileMode) (*os.File, bool, error) {
	exists := false
	file, errf := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, mode)
	if errf != nil {
		if !os.IsExist(errf) {
			return nil, false, errf
		}
		exists = true
		file, errf = os.OpenFile(path, os.O_CREATE|os.O_RDWR, mode)
		if errf != nil {
			return nil, false, errf
		}
	}
	if errl := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errl != nil {
		if errl == syscall.EWOULDBLOCK {
			return nil, exists, ErrLockTaken
		}
		return nil, exists, errl
	}
	return file, exists, nil
}
