package cmd

import (
	"errors"
	"strings"

	"github.com/openshift/geard/transport"
)

type Locator interface {
	// The location of this resource
	TransportLocator() transport.Locator
	// Two resources are identical if they have the same
	// identity value.
	Identity() string
}
type Locators []Locator

// Group resource locators by their transport location
func (locators Locators) Group() (remote []Locators) {
	groups := make(map[string]Locators)
	for i := range locators {
		locator := locators[i].TransportLocator()
		remotes, ok := groups[locator.String()]
		if !ok {
			remotes = make(Locators, 0, 2)
		}
		groups[locator.String()] = append(remotes, locators[i])
	}
	remote = make([]Locators, 0, len(groups))
	for k := range groups {
		remotes := groups[k]
		remote = append(remote, remotes)
	}
	return
}

/*
func (locators Locators) Has(locator Locator) bool {
	for i := range locators {
		if locators[i].Identity() == locator.Identity() {
			return true
		}
	}
	return false
}
*/

// Disambiguate resources with the same id via their type
type ResourceType string

type ResourceValidator interface {
	Type() ResourceType
}

// The resource on a server reachable via a transport.
type ResourceLocator struct {
	// The type of resource being referenced
	Type ResourceType
	// The identifier of the resource at this location
	Id string
	// A label representing a server (varies by transport)
	At transport.Locator
}
type ResourceLocators []ResourceLocator

func (r *ResourceLocator) TransportLocator() transport.Locator {
	return r.At
}
func (r *ResourceLocator) Identity() string {
	base := r.Id
	if r.At != transport.Local {
		base = r.At.String() + "/" + base
	}
	if r.Type != "" {
		base = string(r.Type) + "://" + base
	}
	return base
}

func NewResourceLocators(t transport.Transport, defaultType ResourceType, values ...string) (Locators, error) {
	out := make(Locators, 0, len(values))
	for i := range values {
		r, err := NewResourceLocator(t, defaultType, values[i])
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}
func NewResourceLocator(t transport.Transport, defaultType ResourceType, value string) (Locator, error) {
	res, host, id, errs := SplitTypeHostSuffix(value)
	if errs != nil {
		return nil, errs
	}
	if res == "" {
		res = defaultType
	}
	locator, err := t.LocatorFor(host)
	if err != nil {
		return nil, err
	}
	return &ResourceLocator{ResourceType(res), id, locator}, nil
}

// Given a command line string representing a resource, break it into type, host identity, and suffix
func SplitTypeHostSuffix(value string) (res ResourceType, host string, suffix string, err error) {
	if value == "" {
		err = errors.New("The identifier must be specified as <host>/<id> or <id>")
		return
	}

	locatorParts := strings.SplitN(value, "://", 2)
	if len(locatorParts) == 2 {
		res = ResourceType(locatorParts[0])
		value = locatorParts[1]
	}

	sections := strings.SplitN(value, "/", 2)
	if len(sections) == 1 {
		suffix = sections[0]
		return
	}
	if strings.TrimSpace(sections[0]) == "" {
		err = errors.New("You must specify <host>/<id> or <id>")
		return
	}
	host = sections[0]
	suffix = sections[1]
	return
}

func NewHostLocators(t transport.Transport, values ...string) (Locators, error) {
	out := make(Locators, 0, len(values))
	for i := range values {
		r, err := t.LocatorFor(values[i])
		if err != nil {
			return out, err
		}
		out = append(out, &ResourceLocator{"", "", r})
	}
	return out, nil
}
