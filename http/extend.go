package http

import (
	"github.com/openshift/geard/transport"
)

func init() {
	transport.RegisterTransport("http", NewHttpTransport())
}
