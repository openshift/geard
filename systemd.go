package geard

import (
	"github.com/smarterclayton/go-systemd/dbus"
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
}

var connection Systemd

func StartSystemdConnection() error {
	conn, err := dbus.New()
	if err != nil {
		connection = NewStubSystemd()
		return err
	}
	connection = conn
	return nil
}

func SystemdConnection() Systemd {
	return connection
}
