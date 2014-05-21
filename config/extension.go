package config

import (
	"errors"
	"os"
	"sort"
	"sync"

	"github.com/openshift/geard/selinux"
)

// Describe a directory that should be required to exist
func AddRequiredDirectory(mode os.FileMode, paths ...string) {
	dirLock.Lock()
	defer dirLock.Unlock()
	for i := range paths {
		requiredDirectories = append(requiredDirectories, directoryCheck{paths[i], mode, false})
	}
}

// Check any directories that have not yet been validated.
func HasRequiredDirectories() error {
	dirLock.Lock()
	defer dirLock.Unlock()
	if dirError != nil {
		return dirError
	}
	// ensure parent permissions are initialized before children
	sort.Sort(requiredDirectories)
	for i := range requiredDirectories {
		info := &requiredDirectories[i]
		if info.checked {
			continue
		}
		if err := checkPath(info.path, info.mode, true); err != nil {
			dirError = err
			return err
		}
		info.checked = true
	}
	return nil
}

func checkPath(path string, mode os.FileMode, dir bool) error {
	stat, err := os.Lstat(path)
	if os.IsNotExist(err) && dir {
		err = os.MkdirAll(path, mode)
		stat, _ = os.Lstat(path)
	}
	if err != nil {
		return errors.New("init: path (" + path + ") could not be created: " + err.Error())
	}
	if stat.IsDir() != dir {
		return errors.New("init: path (" + path + ") must be a directory instead of a file")
	}
	if err := selinux.RestoreCon(path, false); err != nil {
		return err
	}
	return nil
}

type directoryCheck struct {
	path    string
	mode    os.FileMode
	checked bool
}
type directoryChecks []directoryCheck

func (a directoryChecks) Len() int           { return len(a) }
func (a directoryChecks) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a directoryChecks) Less(i, j int) bool { return a[i].path < a[j].path }

var dirLock sync.Mutex
var requiredDirectories directoryChecks
var dirError error
