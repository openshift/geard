package transport

import (
	"errors"
	"log"

	"github.com/openshift/geard/jobs"
)

var ErrNotTransportable = errors.New("The specified job cannot be executed remotely")

// Allow Jobs to be remotely executed.
type Transport interface {
	// Return a locator from the given string
	LocatorFor(string) (Locator, error)
	// Given a locator, return a job that can be executed
	// remotely.  May return ErrNotTransportable or
	// ErrNoJobForRequest
	RemoteJobFor(Locator, interface{}) (jobs.Job, error)
}

var transports = make(map[string]Transport)

// Define the implementation of a transport for use
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
