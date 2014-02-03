package geard

import (
	"github.com/smarterclayton/go-systemd/dbus"
)

var connection *dbus.Conn

func StartSystemdConnection() error {
	conn, err := dbus.New()
	if err != nil {
		return err
	}
	connection = conn
	return nil
}

func SystemdConnection() *dbus.Conn {
	return connection
}
