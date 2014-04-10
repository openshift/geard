package router

import (
	"fmt"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/utils"
	"io"
	"path/filepath"
	"strings"
)

const (
	ProtocolHttp  = "http"
	ProtocolHttps = "https"
	ProtocolTls   = "tls"
)

type Identifier string

type Backend struct {
	Id      Identifier
	Servers Servers
}

type Frontend struct {
	Host        string
	Path        string
	Protocols   []string
	Certificate *Certificate
}

type Certificate struct {
	Id                 Identifier
	Contents           []byte
	PrivateKey         []byte
	PrivateKeyPassword string
}
type Certificates []Certificate

type Server struct {
	Id    Identifier
	Host  string
	Ports Ports
}
type Servers []Server

type Port struct {
	Port      port.Port
	Protocols []string
}
type Ports []Port

func (i Identifier) BackendPathfor() string {
	return utils.IsolateContentPath(filepath.Join(config.ContainerBasePath(), "routes", "backends"), string(i), "")
}

func (f *Frontend) Remove() {
}

func (b *Backend) WriteTo(w io.Writer) error {
	for i := range b.Servers {
		server := &b.Servers[i]
		for j := range server.Ports {
			port := &server.Ports[j]
			if _, err := fmt.Fprintf(w, "%d\t%s\t%s\t%s", port.Port, server.Id, server.Host, strings.Join(port.Protocols, ",")); err != nil {
				return err
			}
		}
	}
	return nil
}
