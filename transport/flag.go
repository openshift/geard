package transport

import (
	"errors"
	"fmt"
)

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
