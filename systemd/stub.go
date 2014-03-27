package systemd

import (
	"errors"
	dbus "github.com/openshift/go-systemd/dbus"
	"log"
	"time"
)

// Stub of Systemd interface
type StubSystemd struct {
}

func NewStubSystemd() *StubSystemd {
	return &StubSystemd{}
}

func (c *StubSystemd) LoadUnit(name string) (string, error) {
	return "done", nil
}

func (c *StubSystemd) StartUnit(name string, mode string) (string, error) {
	log.Print("stub_systemd: StartUnit", name, mode)
	return "done", nil
}

func (c *StubSystemd) StartUnitJob(name string, mode string) error {
	log.Print("stub_systemd: StartUnitJob", name, mode)
	return nil
}

func (c *StubSystemd) StopUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (c *StubSystemd) StopUnitJob(name string, mode string) error {
	log.Print("stub_systemd: StopUnitJob", name, mode)
	return nil
}

func (c *StubSystemd) ReloadUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (c *StubSystemd) RestartUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (c *StubSystemd) RestartUnitJob(name string, mode string) error {
	log.Print("stub_systemd: RestartUnitJob", name, mode)
	return nil
}

func (c *StubSystemd) TryRestartUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (c *StubSystemd) ReloadOrRestartUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (c *StubSystemd) ReloadOrTryRestartUnit(name string, mode string) (string, error) {
	return "", errors.New("Not implemented")
}

func (c *StubSystemd) StartTransientUnit(name string, mode string, properties ...dbus.Property) (string, error) {
	return "", errors.New("Not implemented")
}

func (c *StubSystemd) KillUnit(name string, signal int32) {
}

func (c *StubSystemd) GetUnitProperties(unit string) (map[string]interface{}, error) {
	return nil, errors.New("Not implemented")
}

func (c *StubSystemd) SetUnitProperties(unit string, runtime bool, properties ...dbus.Property) error {
	return errors.New("Not implemented")
}

func (c *StubSystemd) ListUnits() ([]dbus.UnitStatus, error) {
	return nil, errors.New("Not implemented")
}

func (c *StubSystemd) EnableUnitFiles(files []string, runtime bool, force bool) (bool, []dbus.EnableUnitFileChange, error) {
	log.Print("stub_systemd: EnableUnitFiles", files, runtime, force)
	return true, nil, nil
}

func (c *StubSystemd) DisableUnitFiles(files []string, runtime bool) ([]dbus.DisableUnitFileChange, error) {
	log.Print("stub_systemd: DisableUnitFiles", files, runtime)
	return nil, nil
}

func (c *StubSystemd) Subscribe() error {
	return nil
}

func (c *StubSystemd) Unsubscribe() error {
	return nil
}

func (c *StubSystemd) SubscribeUnits(interval time.Duration) (<-chan map[string]*dbus.UnitStatus, <-chan error) {
	return c.SubscribeUnitsCustom(interval, 0, func(u1, u2 *dbus.UnitStatus) bool { return *u1 != *u2 }, nil)
}

// SubscribeUnitsCustom is like SubscribeUnits but lets you specify the buffer
// size of the channels, the comparison function for detecting changes and a filter
// function for cutting down on the noise that your channel receives.
func (c *StubSystemd) SubscribeUnitsCustom(interval time.Duration, buffer int, isChanged func(*dbus.UnitStatus, *dbus.UnitStatus) bool, filterUnit func(string) bool) (<-chan map[string]*dbus.UnitStatus, <-chan error) {
	statusChan := make(chan map[string]*dbus.UnitStatus, buffer)
	errChan := make(chan error, buffer)
	close(statusChan)
	close(errChan)
	return statusChan, errChan
}

func (c *StubSystemd) Reload() error {
	return nil
}
