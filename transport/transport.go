package transport

import (
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"

	"log"
	"reflect"
)

type ResourceLocator interface {
	Identifier() containers.Identifier
}

type RemoteExecutable interface{}

type TransportRequest interface {
	RemoteExecutable
	jobs.Job
}

type Dispatcher interface {
	Dispatch(job RemoteExecutable, res jobs.JobResponse) error
}

type Transport interface {
	NewDispatcher(locator ResourceLocator, logger *log.Logger) (Dispatcher, error)
	RequestFor(job jobs.Job) TransportRequest
}

var transports = make(map[string]Transport)

func RegisterTransport(name string, t Transport) {
	log.Printf("register transport %v, %v", name, reflect.TypeOf(t))
	transports[name] = t
}

func GetTransport(name string) Transport {
	log.Printf("get transport %v", name)
	return transports[name]
}

func GetTransportNames() []string {
	return []string{"http"}
}
