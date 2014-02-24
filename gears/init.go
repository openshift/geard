package gears

import (
	"errors"
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/selinux"
	"github.com/smarterclayton/geard/systemd"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
)

func InitializeData() error {
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
	if err := initializeBinaries(); err != nil {
		log.Printf("WARNING: Unable to setup binaries - some operations may not be available: %v", err)
		return err
	}
	return nil
}

func Clean() {
	disableAllUnits()
}

func verifyDataPaths() error {
	for _, path := range []string{
		config.GearBasePath(),
		filepath.Join(config.GearBasePath(), "home"),
		filepath.Join(config.GearBasePath(), "bin"),
	} {
		if err := checkPath(path, os.FileMode(0775), true); err != nil {
			return err
		}
		if err := selinux.RestoreCon(path, false); err != nil {
			return err
		}
	}
	for _, path := range []string{
		filepath.Join(config.GearBasePath(), "targets"),
		filepath.Join(config.GearBasePath(), "units"),
		filepath.Join(config.GearBasePath(), "slices"),
		filepath.Join(config.GearBasePath(), "git"),
		filepath.Join(config.GearBasePath(), "env", "contents"),
		filepath.Join(config.GearBasePath(), "access", "git", "read"),
		filepath.Join(config.GearBasePath(), "access", "git", "write"),
		filepath.Join(config.GearBasePath(), "access", "gears", "ssh"),
		filepath.Join(config.GearBasePath(), "keys", "public"),
		filepath.Join(config.GearBasePath(), "ports", "descriptions"),
		filepath.Join(config.GearBasePath(), "ports", "interfaces"),
	} {
		if err := checkPath(path, os.FileMode(0770), true); err != nil {
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
		[]string{"gear", "multi-user.target"},
	} {
		name, wants := target[0], target[1]
		path := filepath.Join(config.GearBasePath(), "targets", name+".target")
		unit, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
		if os.IsExist(err) {
			continue
		} else if err != nil {
			return err
		}

		if errs := systemd.TargetUnitTemplate.Execute(unit, systemd.TargetUnit{name, wants}); errs != nil {
			log.Printf("gear: Unable to write target %s: %v", name, errs)
			continue
		}
		if errc := unit.Close(); errc != nil {
			log.Printf("gear: Unable to close target %s: %v", name, errc)
			continue
		}

		if _, errs := systemd.StartAndEnableUnit(systemd.SystemdConnection(), name+".target", path, "fail"); errs != nil {
			log.Printf("gear: Unable to start and enable target %s: %v", name, errs)
			continue
		}
	}
	return nil
}

func initializeSlices() error {
	for _, name := range []string{
		"gear",
		"gear-small",
	} {
		path := filepath.Join(config.GearBasePath(), "slices", name+".slice")
		unit, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
		if os.IsExist(err) {
			continue
		} else if err != nil {
			return err
		}

		parent := "gear"
		if name == "gear" {
			parent = ""
		}
		if errs := systemd.SliceUnitTemplate.Execute(unit, systemd.SliceUnit{name, parent}); errs != nil {
			log.Printf("gear: Unable to write slice %s: %v", name, errs)
			continue
		}
		if errc := unit.Close(); errc != nil {
			log.Printf("gear: Unable to close slice %s: %v", name, errc)
			continue
		}

		if _, errs := systemd.StartAndEnableUnit(systemd.SystemdConnection(), name+".slice", path, "fail"); errs != nil {
			log.Printf("gear: Unable to start and enable slice %s: %v", name, errs)
			continue
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
		return errors.New("gear: path (" + path + ") is not available: " + err.Error())
	}
	if stat.IsDir() != dir {
		return errors.New("gear: path (" + path + ") must be a directory instead of a file")
	}
	return nil
}

func disableAllUnits() {
	systemd := systemd.SystemdConnection()

	for _, path := range []string{
		filepath.Join(config.GearBasePath(), "units"),
		filepath.Join(config.GearBasePath(), "slices"),
		filepath.Join(config.GearBasePath(), "targets"),
	} {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				log.Printf("gear: Can't read %s: %v", p, err)
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if _, err := systemd.DisableUnitFiles([]string{p}, false); err != nil {
				log.Printf("gear: Unable to disable %s: %+v", p, err)
			}
			return nil
		})
		if err := systemd.Reload(); err != nil {
			log.Printf("gear: systemd reload failed: %+v", err)
		}
	}
}

func copyBinary(src string, dest string, setUid bool) error {
	var err error
	var sourceInfo os.FileInfo
	if sourceInfo, err = os.Stat(src); err != nil {
		return err
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("Cannot copy source %s", src)
	}

	if _, err = os.Stat(dest); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if err = os.Remove(dest); err != nil {
			return err
		}
	}

	var mode os.FileMode
	if setUid {
		mode = 0555 | os.ModeSetuid
	} else {
		mode = 0555
	}

	var destFile *os.File
	if destFile, err = os.Create(dest); err != nil {
		return err
	}
	defer destFile.Close()

	var sourceFile *os.File
	if sourceFile, err = os.Open(src); err != nil {
		return err
	}
	defer sourceFile.Close()

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}
	destFile.Sync()

	if err = destFile.Chmod(mode); err != nil {
		return err
	}

	return nil
}

func HasBinaries() bool {
	for _,b := range []string{
		path.Join(config.GearBasePath(), "bin", "switchns"),
		path.Join(config.GearBasePath(), "bin", "gear"),
	} {
		if _, err := os.Stat(b) ; err != nil {
			return false
		}
	}
	return true
}

func initializeBinaries() error {
	destDir := path.Join(config.GearBasePath(), "bin")
	srcDir := path.Join("/", "opt", "geard", "bin")
	if err := copyBinary(path.Join(srcDir, "switchns"), path.Join(destDir, "switchns"), true); err != nil {
		return err
	}
	
	srcDir = path.Join("/", "bin")	
	if err := copyBinary(path.Join(srcDir, "gear"), path.Join(destDir, "gear"), false); err != nil {
		return err
	}
	return nil
}
