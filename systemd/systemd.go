package systemd

import (
	"encoding/base64"
	"fmt"
	db "github.com/guelfey/go.dbus"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/go-systemd/dbus"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"time"
)

type SystemdFileType string

const (
	TargetType SystemdFileType = "target"
	SliceType  SystemdFileType = "slice"
	UnitType   SystemdFileType = "unit"
)

type Systemd interface {
	LoadUnit(name string) (string, error)
	StartUnit(name string, mode string) (string, error)
	StartUnitJob(name string, mode string) error
	StopUnit(name string, mode string) (string, error)
	StopUnitJob(name string, mode string) error
	ReloadUnit(name string, mode string) (string, error)
	RestartUnit(name string, mode string) (string, error)
	RestartUnitJob(name string, mode string) error
	TryRestartUnit(name string, mode string) (string, error)
	ReloadOrRestartUnit(name string, mode string) (string, error)
	ReloadOrTryRestartUnit(name string, mode string) (string, error)
	StartTransientUnit(name string, mode string, properties ...dbus.Property) (string, error)
	KillUnit(name string, signal int32)
	GetUnitProperties(unit string) (map[string]interface{}, error)
	SetUnitProperties(name string, runtime bool, properties ...dbus.Property) error
	ListUnits() ([]dbus.UnitStatus, error)
	EnableUnitFiles(files []string, runtime bool, force bool) (bool, []dbus.EnableUnitFileChange, error)
	DisableUnitFiles(files []string, runtime bool) ([]dbus.DisableUnitFileChange, error)

	Subscribe() error
	Unsubscribe() error
	SubscribeUnits(time.Duration) (<-chan map[string]*dbus.UnitStatus, <-chan error)
	SubscribeUnitsCustom(time.Duration, int, func(*dbus.UnitStatus, *dbus.UnitStatus) bool, func(string) bool) (<-chan map[string]*dbus.UnitStatus, <-chan error)

	Reload() error
}

func InitializeSystemdFile(fType SystemdFileType, name string, template *template.Template, values interface{}) error {
	var partPath string
	var ext string

	switch {
	case fType == TargetType:
		partPath = "targets"
		ext = ".target"
	case fType == SliceType:
		partPath = "slices"
		ext = ".slice"
	case fType == UnitType:
		partPath = "units"
		ext = ".service"
	}

	path := filepath.Join(config.ContainerBasePath(), partPath, name+ext)
	unit, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if os.IsExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	if errs := template.Execute(unit, values); errs != nil {
		log.Printf("gear: Unable to write %s %s: %v", fType, name, errs)
		return nil
	}
	if errc := unit.Close(); errc != nil {
		log.Printf("gear: Unable to close target %s %s: %v", fType, name, errc)
		return nil
	}

	if _, errs := StartAndEnableUnit(Connection(), name+ext, path, "fail"); errs != nil {
		log.Printf("gear: Unable to start and enable %s %s: %v", fType, name, errs)
	}

	return nil
}

func StartAndEnableUnit(systemd Systemd, name, path, mode string) (string, error) {
	status, err := systemd.StartUnit(name, mode)
	switch {
	case IsNoSuchUnit(err), IsLoadFailed(err):
		if _, err := os.Stat(path); err != nil {
			return "", ErrNoSuchUnit
		}
		if err := EnableAndReloadUnit(systemd, name, path); err != nil {
			return "", err
		}
		return systemd.StartUnit(name, mode)
	}
	return status, err
}

func EnableAndReloadUnit(systemd Systemd, name string, path ...string) error {
	if _, _, err := systemd.EnableUnitFiles(path, false, true); err != nil {
		return err
	}
	if ok, err := IsUnitProperty(systemd, name, func(p map[string]interface{}) bool {
		return p["LoadState"] == "not-found" || p["NeedDaemonReload"] == true
	}); err == nil && ok {
		// The daemon needs to be reloaded to pick up the changed configuration
		log.Printf("systemd: Reloading daemon")
		if errr := systemd.Reload(); errr != nil {
			log.Printf("systemd: Contents changed on disk and reload failed, subsequent start will likely fail: %v", errr)
		}
	}
	return nil
}

func SafeUnitName(r []byte) string {
	return strings.Trim(base64.URLEncoding.EncodeToString(r), "=")
}

var connection Systemd

func NewConnection() (Systemd, error) {
	conn, err := dbus.New()
	if err != nil {
		return NewStubSystemd(), err
	}
	return conn, nil
}

func StartConnection() error {
	if connection == nil {
		conn, err := NewConnection()
		if err != nil {
			connection = conn
			return err
		}
		connection = conn
	}
	return nil
}

func Start() error {
	if err := StartConnection(); err != nil {
		log.Println("WARNING: No systemd connection available via dbus: ", err)
		log.Println("  You may need to run as root or check that /var/run/dbus/system_bus_socket is bind mounted.")
		return err
	}
	return nil
}

func Require() {
	if err := Start(); err != nil {
		os.Exit(1)
	}
}

func Connection() Systemd {
	return connection
}

func SystemdError(err error, name string) bool {
	if errd, ok := err.(db.Error); ok {
		return errd.Name == name
	}
	return false
}

func SprintSystemdError(err error) string {
	if errd, ok := err.(db.Error); ok {
		return fmt.Sprintf("%s %s", reflect.TypeOf(errd), errd.Name)
	}
	return err.Error()
}

var ErrNoSuchUnit = db.Error{Name: "org.freedesktop.systemd1.NoSuchUnit"}

func IsUnitProperty(systemd Systemd, unit string, f func(p map[string]interface{}) bool) (bool, error) {
	p, err := systemd.GetUnitProperties(unit)
	if err != nil {
		log.Printf("debug: Found error while checking unit state %s: %v", unit, err)
		return false, err
	}
	return f(p), nil
}

func IsUnitLoadState(systemd Systemd, unit string, state string) (bool, error) {
	return IsUnitProperty(systemd, unit, func(p map[string]interface{}) bool {
		return p["LoadState"] == state
	})
}

func IsNoSuchUnit(err error) bool {
	return SystemdError(err, "org.freedesktop.systemd1.NoSuchUnit")
}
func IsFileNotFound(err error) bool {
	return SystemdError(err, "org.freedesktop.DBus.Error.FileNotFound")
}
func IsLoadFailed(err error) bool {
	return SystemdError(err, "org.freedesktop.systemd1.LoadFailed")
}

func WriteStatusTo(w io.Writer, unit string) error {
	cmd := exec.Command("/usr/bin/systemctl", "status", unit)
	cmd.Stdout = w
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}
