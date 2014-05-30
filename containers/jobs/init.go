// +build linux

package jobs

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/openshift/geard/config"
	csystemd "github.com/openshift/geard/containers/systemd"
	"github.com/openshift/geard/systemd"
)

func InitializeData() error {
	syscall.Umask(0000)
	if err := initializeTargets(); err != nil {
		return err
	}
	if err := initializeSlices(); err != nil {
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

func initializeTargets() error {
	for _, target := range [][]string{
		{"container", ""},
		{"container-sockets", ""},
		{"container-active", "multi-user.target"},
	} {
		name, wants := target[0], target[1]
		if err := systemd.InitializeSystemdFile(systemd.TargetType, name, csystemd.TargetUnitTemplate, csystemd.TargetUnit{name, wants}, false); err != nil {
			return err
		}
	}
	return nil
}

const (
	DefaultSlice string = "container-small"
)

var (
	sliceUnits = []csystemd.SliceUnit{
		{"container", "", "512M"},
		{DefaultSlice, "container", "512M"},
		{"container-large", "container", "1G"},
	}
)

func ListSliceNames() []string {
	names := []string{}
	for _, unit := range sliceUnits {
		names = append(names, unit.Name)
	}
	return names
}

func initializeSlices() error {
	for _, unit := range sliceUnits {
		if err := systemd.InitializeSystemdFile(systemd.SliceType, unit.Name, csystemd.SliceUnitTemplate, unit, false); nil != err {
			return err
		}
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
