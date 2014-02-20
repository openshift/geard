package systemd

import (
	"fmt"
	db "github.com/guelfey/go.dbus"
	"github.com/smarterclayton/go-systemd/dbus"
	"log"
	"os"
	"reflect"
	"time"
)

type Systemd interface {
	LoadUnit(name string) (string, error)
	StartUnit(name string, mode string) (string, error)
	StopUnit(name string, mode string) (string, error)
	ReloadUnit(name string, mode string) (string, error)
	RestartUnit(name string, mode string) (string, error)
	TryRestartUnit(name string, mode string) (string, error)
	ReloadOrRestartUnit(name string, mode string) (string, error)
	ReloadOrTryRestartUnit(name string, mode string) (string, error)
	StartTransientUnit(name string, mode string, properties ...dbus.Property) (string, error)
	KillUnit(name string, signal int32)
	GetUnitProperties(unit string) (map[string]interface{}, error)
	ListUnits() ([]dbus.UnitStatus, error)
	EnableUnitFiles(files []string, runtime bool, force bool) (bool, []dbus.EnableUnitFileChange, error)
	DisableUnitFiles(files []string, runtime bool) ([]dbus.DisableUnitFileChange, error)

	Subscribe() error
	Unsubscribe() error
	SubscribeUnits(time.Duration) (<-chan map[string]*dbus.UnitStatus, <-chan error)
	SubscribeUnitsCustom(time.Duration, int, func(*dbus.UnitStatus, *dbus.UnitStatus) bool, func(string) bool) (<-chan map[string]*dbus.UnitStatus, <-chan error)

	Reload() error
}

func StartAndEnableUnit(systemd Systemd, name, path, mode string) (string, error) {
	status, err := systemd.StartUnit(name, mode)
	switch {
	case IsNoSuchUnit(err), IsLoadFailed(err):
		if _, err := os.Stat(path); err != nil {
			return "", ErrNoSuchUnit
		}
		if _, _, err := systemd.EnableUnitFiles([]string{path}, false, true); err != nil {
			return "", err
		}
		if ok, err := IsUnitLoadState(systemd, name, "not-found"); err == nil && ok {
			// The daemon needs to be reloaded to pick up the changed configuration
			if errr := systemd.Reload(); errr != nil {
				log.Printf("systemd: Contents changed on disk and reload failed, subsequent start will likely fail: %v", errr)
			}
		}
		return systemd.StartUnit(name, mode)
	}
	return status, err
}

type ProvidesUnitName interface {
	UnitNameFor() string
}

var connection Systemd

func NewSystemdConnection() (Systemd, error) {
	conn, err := dbus.New()
	if err != nil {
		return NewStubSystemd(), err
	}
	return conn, nil
}

func StartSystemdConnection() error {
	conn, err := NewSystemdConnection()
	if err != nil {
		return err
	}
	connection = conn
	return nil
}

func SystemdConnection() Systemd {
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

func IsUnitLoadState(systemd Systemd, unit string, state string) (bool, error) {
	p, err := systemd.GetUnitProperties(unit)
	if err != nil {
		log.Printf("debug: Found error while checking unit state %s: %v", unit, err)
		return false, err
	}
	return p["LoadState"] == state, nil
}

func IsNoSuchUnit(err error) bool {
	return SystemdError(err, "org.freedesktop.systemd1.NoSuchUnit")
}
func IsLoadFailed(err error) bool {
	return SystemdError(err, "org.freedesktop.systemd1.LoadFailed")
}
