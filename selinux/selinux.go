// +build !noselinux

package selinux

import (
	se "github.com/rhatdan/selinux"
	"path"
	"path/filepath"
)

func RestoreCon(path string) error {
	var flabel string
	var err error

	if !se.Selinux_enabled() {
		return nil
	}

	if flabel, err = se.Matchpathcon(path, 0); err != nil {
		return nil
	}

	if rc, err := se.Setfilecon(path, flabel); rc != 0 {
		return err
	}

	return nil
}

func RestoreConRecursive(basePath string) error {
	var paths []string
	var err error

	if paths, err = filepath.Glob(path.Join(basePath, "**", "*")); err != nil {
		return err
	}

	for _, path := range paths {
		if err = RestoreCon(path); err != nil {
			return err
		}
	}

	return nil
}
