package transport

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/openshift/geard/port"
)

// The reserved identifier for the local transport
const localTransport = "local"

// The destination of a transport.  All transports
// must provide a way to resolve the IP remote hostname.
type Locator interface {
	// A string that uniquely identifies this destination
	String() string
	// Return a valid hostname for this locator
	ResolveHostname() (string, error)
}
type Locators []Locator

// The local transport - test against this variable for
// finding a local interface.
var Local = HostLocator(localTransport)

// A host port combination representing a remote server - most IP
// transports can use this default implementation.
type HostLocator string

func (t HostLocator) String() string {
	return string(t)
}
func (t HostLocator) IsRemote() bool {
	return localTransport != t.String() && "" != t.String()
}
func (t HostLocator) ResolveHostname() (string, error) {
	return ResolveLocatorHostname(t.String())
}

// Return an object representing an IP host
func NewHostLocator(value string) (HostLocator, error) {
	if strings.Contains(value, "/") {
		return "", errors.New("Host identifiers may not have a slash")
	}
	if value == "" || value == localTransport {
		return Local, nil
	}

	if strings.Contains(value, ":") {
		_, portString, err := net.SplitHostPort(value)
		if err != nil {
			return "", err
		}
		if portString != "" {
			p, err := strconv.Atoi(portString)
			if err != nil {
				return "", err
			}
			port := port.Port(p)
			if err := port.Check(); err != nil {
				return "", err
			}
		}
	}
	return HostLocator(value), nil
}

func ResolveLocatorHostname(value string) (string, error) {
	if value != "" && value != localTransport {
		if strings.Contains(value, ":") {
			host, _, err := net.SplitHostPort(value)
			if err != nil {
				return "", err
			}
			return host, nil
		}
		return value, nil
	}
	return "localhost", nil
}

// Convenience method for converting a set of strings into a list of Locators
func NewTransportLocators(transport Transport, values ...string) (Locators, error) {
	out := make(Locators, 0, len(values))
	for i := range values {
		r, err := transport.LocatorFor(values[i])
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}
