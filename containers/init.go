// Data model for containers running under systemd - identifiers, port references,
// network links, state, and events.  The templates for each systemd unit type
// are also available here.
package containers

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/selinux"
	"github.com/openshift/geard/systemd"
	"log"
	"os"
	"path/filepath"
	"syscall"
)

func InitializeData() error {
	syscall.Umask(0000)
	if err := verifyDataPaths(); err != nil {
		log.Fatal(err)
		return err
	}
	if err := initializeTargets(); err != nil {
		log.Fatal(err)
		return err
	}
	if err := initializeSlices(); err != nil {
		log.Fatal(err)
		return err
	}
	if err := checkBinaries(); err != nil {
		log.Printf("WARNING: Unable to find all required binaries - some operations may not be available: %v", err)
	}
	return nil
}

func Clean() {
	disableAllUnits()
}

func verifyDataPaths() error {
	for _, path := range []string{
		config.ContainerBasePath(),
		filepath.Join(config.ContainerBasePath(), "home"),
		filepath.Join(config.ContainerBasePath(), "git"),
		filepath.Join(config.ContainerBasePath(), "units"),
		filepath.Join(config.ContainerBasePath(), "access", "git"),
		filepath.Join(config.ContainerBasePath(), "access", "containers", "ssh"),
		filepath.Join(config.ContainerBasePath(), "keys", "public"),
	} {
		if err := checkPath(path, os.FileMode(0755), true); err != nil {
			return err
		}
		if err := selinux.RestoreCon(path, false); err != nil {
			return err
		}
	}
	for _, path := range []string{
		filepath.Join(config.ContainerBasePath(), "targets"),
		filepath.Join(config.ContainerBasePath(), "slices"),
		filepath.Join(config.ContainerBasePath(), "env", "contents"),
		filepath.Join(config.ContainerBasePath(), "ports", "descriptions"),
		filepath.Join(config.ContainerBasePath(), "ports", "interfaces"),
	} {
		if err := checkPath(path, os.FileMode(0750), true); err != nil {
			return err
		}
		if err := selinux.RestoreCon(path, false); err != nil {
			return err
		}
	}

	return nil
}

func initializeTargets() error {
	for _, target := range [][]string{
		{"container", ""},
		{"container-sockets", ""},
		{"container-active", "multi-user.target"},
	} {
		name, wants := target[0], target[1]
		if err := systemd.InitializeSystemdFile(systemd.TargetType, name, TargetUnitTemplate, TargetUnit{name, wants}, false); err != nil {
			return err
		}
	}
	return nil
}

func initializeSlices() error {
	for _, name := range []string{
		"container",
		"container-small",
	} {
		parent := "container"
		if name == "container" {
			parent = ""
		}

		if err := systemd.InitializeSystemdFile(systemd.SliceType, name, SliceUnitTemplate, SliceUnit{name, parent}, false); err != nil {
			return err
		}
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
		return errors.New("init: path (" + path + ") is not available: " + err.Error())
	}
	if stat.IsDir() != dir {
		return errors.New("init: path (" + path + ") must be a directory instead of a file")
	}
	return nil
}

func isSystemdFile(filePath string) bool {
	extention := filepath.Ext(filePath)
	systemdExts := []string{".slice", ".service", ".socket", ".target"}
	for _, e := range systemdExts {
		if e == extention {
			return true
		}
	}
	return false
}

func disableAllUnits() {
	systemd := systemd.Connection()

	for _, path := range []string{
		filepath.Join(config.ContainerBasePath(), "units"),
		filepath.Join(config.ContainerBasePath(), "slices"),
		filepath.Join(config.ContainerBasePath(), "targets"),
	} {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				log.Printf("init: Can't read %s: %v", p, err)
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if !isSystemdFile(p) {
				return nil
			}
			fmt.Printf("Stopping and disabling %s\n", filepath.Base(p))
			if status, err := systemd.StopUnit(filepath.Base(p), "fail"); err != nil {
				log.Printf("init: Unable to stop %s: %v, %+v", p, status, err)
			}
			if _, err := systemd.DisableUnitFiles([]string{p}, false); err != nil {
				log.Printf("init: Unable to disable %s: %+v", p, err)
			}
			return nil
		})
		if err := systemd.Reload(); err != nil {
			log.Printf("init: systemd reload failed: %+v", err)
		}
	}
}

func checkBinaries() error {
	expectedBinaries := []string{"/usr/bin/gear", "/usr/bin/switchns", "/usr/sbin/gear-auth-keys-command"}

	for _, b := range expectedBinaries {
		if _, err := os.Stat(b); err != nil {
			return fmt.Errorf("Unable to find %v", b)
		}
	}
	return nil
}
