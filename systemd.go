package geard

import (
	"fmt"
	db "github.com/guelfey/go.dbus"
	"github.com/smarterclayton/go-systemd/dbus"
	"reflect"
	"time"
)

type Systemd interface {
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

	Subscribe() error
	Unsubscribe() error
	SubscribeUnits(time.Duration) (<-chan map[string]*dbus.UnitStatus, <-chan error)
	SubscribeUnitsCustom(time.Duration, int, func(*dbus.UnitStatus, *dbus.UnitStatus) bool, func(string) bool) (<-chan map[string]*dbus.UnitStatus, <-chan error)
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
	return ""
}

func ErrNoSuchUnit(err error) bool {
	return SystemdError(err, "org.freedesktop.systemd1.NoSuchUnit")
}
