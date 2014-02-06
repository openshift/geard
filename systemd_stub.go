package geard

import (
	"errors"
	dbus "github.com/smarterclayton/go-systemd/dbus"
	"log"
)

// Stub of Systemd interface
type StubSystemd struct {
}

func NewStubSystemd() *StubSystemd {
	return &StubSystemd{}
}

func (_m *StubSystemd) StartUnit(name string, mode string) (string, error) {
	log.Print("stub_systemd: StartUnit", name, mode)
	return "done", nil
}

func (_m *StubSystemd) StopUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (_m *StubSystemd) ReloadUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (_m *StubSystemd) RestartUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (_m *StubSystemd) TryRestartUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (_m *StubSystemd) ReloadOrRestartUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (_m *StubSystemd) ReloadOrTryRestartUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (_m *StubSystemd) StartTransientUnit(name string, mode string, properties ...dbus.Property) (string, error) {
	return "", errors.New("Not implemented")
}

func (_m *StubSystemd) KillUnit(name string, signal int32) {
}

func (_m *StubSystemd) GetUnitProperties(unit string) (map[string]interface{}, error) {
	return nil, errors.New("Not implemented")
}

func (_m *StubSystemd) ListUnits() ([]dbus.UnitStatus, error) {
	return nil, errors.New("Not implemented")
}

func (_m *StubSystemd) EnableUnitFiles(files []string, runtime bool, force bool) (bool, []dbus.EnableUnitFileChange, error) {
	log.Print("stub_systemd: EnableUnitFiles", files, runtime, force)
	return true, nil, nil
}
