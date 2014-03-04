package cmd

import (
	"errors"
	"github.com/smarterclayton/geard/gears"
	"log"
	"net"
	"net/url"
	"strings"
)

type Locator interface {
	IsRemote() bool
	Identity() string
	String() string
}

type Locators []Locator

func (locators Locators) Group() (local Locators, remote []Locators) {
	local = make(Locators, 0, len(locators))
	groups := make(map[string]Locators)
	for i := range locators {
		locator := locators[i]
		if locator.IsRemote() {
			remotes, ok := groups[locator.Identity()]
			if !ok {
				remotes = make(Locators, 0, 2)
			}
			groups[locator.Identity()] = append(remotes, locator)
		} else {
			local = append(local, locator)
		}
	}
	remote = make([]Locators, 0, len(groups))
	for k := range groups {
		remotes := groups[k]
		remote = append(remote, remotes)
	}
	return
}

type RemoteIdentifier struct {
	Id   gears.Identifier
	Host HostIdentifier
}

func (r RemoteIdentifier) IsRemote() bool {
	return r.Host != ""
}
func (r RemoteIdentifier) Identity() string {
	return string(r.Host)
}
func (r RemoteIdentifier) String() string {
	if r.Host != "" {
		return string(r.Host) + "/" + string(r.Id)
	}
	return string(r.Id)
}
func (r RemoteIdentifier) BaseURL() *url.URL {
	uri, err := r.Host.NewURI()
	if err != nil {
		log.Fatal(err)
	}
	return uri
}

type HostIdentifier string

func (h HostIdentifier) NewURI() (*url.URL, error) {
	host, port, err := net.SplitHostPort(string(h))
	if err != nil {
		return nil, err
	}
	if port == "" {
		port = "2223"
	}
	return &url.URL{
		Scheme: "http",
		Host:   host + ":" + port,
	}, nil
}

func NewRemoteIdentifiers(values []string) ([]Locator, error) {
	out := make([]Locator, 0, len(values))
	for i := range values {
		r, err := NewRemoteIdentifier(values[i])
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

func NewRemoteIdentifier(value string) (*RemoteIdentifier, error) {
	if value == "" {
		return nil, errors.New("The remote identifier must be specified as <host>/<id> or <id>")
	}
	sections := strings.SplitN(value, "/", 2)
	if len(sections) == 1 {
		id, err := gears.NewIdentifier(sections[0])
		if err != nil {
			return nil, err
		}
		return &RemoteIdentifier{id, ""}, nil
	}

	id, err := gears.NewIdentifier(sections[1])
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sections[0]) == "" {
		return nil, errors.New("You must specify <host>/<id> or <id>")
	}
	return &RemoteIdentifier{id, HostIdentifier(sections[0])}, nil
}
