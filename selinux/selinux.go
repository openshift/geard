// +build selinux

package selinux

import (
	"fmt"
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

	if flabel, err = se.Matchpathcon(path, 0); flabel == "" {
		return fmt.Errorf("Unable to get context for %v: %v", path, err)
	}

	if rc, err := se.Setfilecon(path, flabel); rc != 0 {
		return fmt.Errorf("Unable to set selinux context for %v: %v", path, err)
	}

	return nil
}

func RestoreConRecursive(basePath string) error {
	var paths []string
	var err error

	if paths, err = filepath.Glob(path.Join(basePath, "**", "*")); err != nil {
		return fmt.Errorf("Unable to find directory %v: %v", basePath, err)
	}

	for _, path := range paths {
		if err = RestoreCon(path); err != nil {
			return fmt.Errorf("Unable to restore selinux context for %v: %v", path, err)
		}
	}

	return nil
}
