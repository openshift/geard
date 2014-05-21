package main

import (
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/transport"
)

type LocalTransportFlag struct {
	transport.TransportFlag
}

func (t *LocalTransportFlag) Set(name string) error {
	if err := t.TransportFlag.Set(name); err != nil {
		return err
	}
	t.Transport = &localTransport{remote: t.Transport}
	return nil
}

// Create a transport that will invoke the default job implementation for a given
// request with a local locator, and pass any other requests to the remote transport.
type localTransport struct {
	remote transport.Transport
}

func (h *localTransport) LocatorFor(value string) (transport.Locator, error) {
	if transport.Local.String() != value {
		return h.remote.LocatorFor(value)
	}
	return transport.Local, nil
}

func (h *localTransport) RemoteJobFor(locator transport.Locator, j interface{}) (job jobs.Job, err error) {
	if locator != transport.Local {
		return h.remote.RemoteJobFor(locator, j)
	}

	job, err = jobs.JobFor(j)
	if err != nil {
		return
	}
	return
}
