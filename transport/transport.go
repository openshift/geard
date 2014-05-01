package transport

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/jobs"
	"log"
)

var ErrNotTransportable = errors.New("The specified job cannot be executed remotely")

// Allow Jobs to be remotely executed.
type Transport interface {
	// Return a locator from the given string
	LocatorFor(string) (Locator, error)
	// Given a locator, return a job that can be executed
	// remotely.
	RemoteJobFor(Locator, jobs.Job) (jobs.Job, error)
}

type noTransport struct{}

func (t *noTransport) LocatorFor(value string) (Locator, error) {
	return NewHostLocator(value)
}
func (t *noTransport) RemoteJobFor(locator Locator, job jobs.Job) (jobs.Job, error) {
	return job, nil
}

var emptyTransport = &noTransport{}
var transports = make(map[string]Transport)

func RegisterTransport(name string, t Transport) {
	if t == nil {
		log.Printf("Transport for '%s' must not be nil", name)
		return
	}
	transports[name] = t
}

func GetTransport(name string) (Transport, bool) {
	t, ok := transports[name]
	return t, ok
}

func GetTransportNames() []string {
	names := make([]string, 0, len(transports))
	for name, _ := range transports {
		names = append(names, name)
	}
	return names
}

// Implement the flag.Value interface for reading a transport
// from a string.
type TransportFlag struct {
	Transport
	name string
}

func (t *TransportFlag) Get() Transport {
	return t.Transport
}

func (t *TransportFlag) String() string {
	return t.name
}

func (t *TransportFlag) Set(s string) error {
	value, ok := GetTransport(s)
	if !ok {
		return errors.New(fmt.Sprintf("No transport defined for '%s'.  Valid transports are %v", s, GetTransportNames()))
	}
	t.name = s
	t.Transport = value
	return nil
}
