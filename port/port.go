// Ports are a limited resource on a host and so must be shared via
// a central allocator.  This package exposes Port - a valid IP
// port value - and methods for allocation and reservation of
// those ports.
package port

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/openshift/geard/config"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

// An IP port, valid from 1 to 65535.  Use 0 for undefined.
type Port uint

const InvalidPort = 0

func NewPortFromString(value string) (Port, error) {
	i, err := strconv.Atoi(value)
	if err != nil {
		return InvalidPort, err
	}
	if i < 0 || i > 65535 {
		return InvalidPort, errors.New("Port values must be between 0 and 65535")
	}
	return Port(i), nil
}

func (p Port) Default() bool {
	return p == InvalidPort
}

func (p Port) Check() error {
	if p < 1 || p > 65535 {
		return errors.New("Port must be between 1 and 65535")
	}
	return nil
}

func (p Port) String() string {
	return strconv.Itoa(int(p))
}

// A host and port combination, where Host may be empty
type HostPort struct {
	Host string `json:"Host,omitempty"`
	Port `json:"Port,omitempty"`
}

func NewHostPort(hostport string) (HostPort, error) {
	host, portString, err := net.SplitHostPort(hostport)
	if err != nil {
		return HostPort{}, err
	}
	port, err := NewPortFromString(portString)
	if err != nil {
		return HostPort{}, err
	}
	return HostPort{host, port}, nil
}

func (hostport HostPort) String() string {
	return net.JoinHostPort(hostport.Host, hostport.Port.String())
}

func (hostport HostPort) Empty() bool {
	return hostport.Port.Default()
}

func (hostport HostPort) Local() bool {
	return hostport.Host == "" || hostport.Host == "127.0.0.1" || hostport.Host == "localhost"
}

type PortPair struct {
	Internal Port
	External Port `json:"External,omitempty"`
}

type PortPairs []PortPair

func (p PortPairs) Find(port Port) (*PortPair, bool) {
	for i := range p {
		if p[i].Internal == port {
			return &p[i], true
		}
	}
	return nil, false
}
func (p PortPairs) ToHeader() string {
	var pairs bytes.Buffer
	for i := range p {
		if i != 0 {
			pairs.WriteString(",")
		}
		pairs.WriteString(strconv.Itoa(int(p[i].Internal)))
		pairs.WriteString(":")
		pairs.WriteString(strconv.Itoa(int(p[i].External)))
	}
	return pairs.String()
}
func (p PortPairs) String() string {
	var pairs bytes.Buffer
	for i := range p {
		if i != 0 {
			pairs.WriteString(", ")
		}
		pairs.WriteString(strconv.Itoa(int(p[i].Internal)))
		pairs.WriteString(" -> ")
		pairs.WriteString(strconv.Itoa(int(p[i].External)))
	}
	return pairs.String()
}

func FromPortPairHeader(s string) (PortPairs, error) {
	pairs := strings.Split(s, ",")
	ports := make(PortPairs, 0, len(pairs))
	for i := range pairs {
		pair := pairs[i]
		value := strings.SplitN(pair, ":", 2)
		if len(value) != 2 {
			return PortPairs{}, errors.New(fmt.Sprintf("The port string '%s' must be a comma delimited list of pairs <internal>:<external>,...", s))
		}
		internal, err := NewPortFromString(value[0])
		if err != nil {
			return PortPairs{}, err
		}
		external, err := NewPortFromString(value[1])
		if err != nil {
			return PortPairs{}, err
		}
		ports = append(ports, PortPair{Port(internal), Port(external)})
	}
	return ports, nil
}

type Device string

func (d Device) DevicePath() string {
	return filepath.Join(config.ContainerBasePath(), "ports", "interfaces", string(d))
}
