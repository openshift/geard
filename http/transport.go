package http

import (
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/openshift/geard/http/client"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/transport"
)

type RemoteLocator interface {
	ToURL() *url.URL
}

type ServerAware interface {
	SetServer(string)
}

type HttpTransport struct {
	client.HttpClient
}

func (h *HttpTransport) LocatorFor(value string) (transport.Locator, error) {
	return transport.NewHostLocator(value)
}

func (h *HttpTransport) RemoteJobFor(locator transport.Locator, j interface{}) (job jobs.Job, err error) {
	baseUrl, errl := urlForLocator(locator)
	if errl != nil {
		err = errors.New("The provided host is not valid '" + locator.String() + "': " + errl.Error())
		return
	}
	httpJob, errh := HttpJobFor(j)
	if errh == jobs.ErrNoJobForRequest {
		err = transport.ErrNotTransportable
		return
	}
	if errh != nil {
		err = errh
		return
	}
	if serverAware, ok := httpJob.(ServerAware); ok {
		serverAware.SetServer(baseUrl.Host)
	}

	job = jobs.JobFunction(func(res jobs.Response) {
		if err := h.ExecuteRemote(baseUrl, httpJob, res); err != nil {
			res.Failure(err)
		}
	})
	return
}

func urlForLocator(locator transport.Locator) (*url.URL, error) {
	base := locator.String()
	if strings.Contains(base, ":") {
		host, port, err := net.SplitHostPort(base)
		if err != nil {
			return nil, err
		}
		if port == "" {
			base = net.JoinHostPort(host, client.DefaultHttpPort)
		}
	} else {
		base = net.JoinHostPort(base, client.DefaultHttpPort)
	}
	return &url.URL{Scheme: "http", Host: base}, nil
}

func HttpJobFor(job interface{}) (exc client.RemoteExecutable, err error) {
	for _, ext := range extensions {
		req, errr := ext.HttpJobFor(job)
		if errr == jobs.ErrNoJobForRequest {
			continue
		}
		if errr != nil {
			return nil, errr
		}
		return req, nil
	}
	err = jobs.ErrNoJobForRequest
	return
}
