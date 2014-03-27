package cmd

import (
	"errors"
	"fmt"
	"github.com/openshift/geard/containers"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	LocalHostName                       = "local"
	ResourceTypeContainer  ResourceType = "ctr"
	ResourceTypeRepository ResourceType = "repo"
)

type ResourceType string

type Locator interface {
	ResourceType() ResourceType
	IsRemote() bool
	Identity() string
	HostIdentity() string
}
type Locators []Locator
type ResourceLocator interface {
	Identifier() containers.Identifier
}

func LocatorsAreEqual(a, b Locator) bool {
	return a.Identity() == b.Identity()
}

func (locators Locators) Has(locator Locator) bool {
	for i := range locators {
		if locators[i].Identity() == locator.Identity() {
			return true
		}
	}
	return false
}

func (locators Locators) Group() (local Locators, remote []Locators) {
	local = make(Locators, 0, len(locators))
	groups := make(map[string]Locators)
	for i := range locators {
		locator := locators[i]
		if locator.IsRemote() {
			remotes, ok := groups[locator.HostIdentity()]
			if !ok {
				remotes = make(Locators, 0, 2)
			}
			groups[locator.HostIdentity()] = append(remotes, locator)
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

type HostLocator struct {
	Host string
	Port containers.Port
}

type ContainerLocator struct {
	HostLocator
	Id containers.Identifier
}

type GenericLocator struct {
	HostLocator
	Id   containers.Identifier
	Type ResourceType
}

func NewHostLocators(values ...string) (Locators, error) {
	out := make(Locators, 0, len(values))
	for i := range values {
		r, err := NewHostLocator(values[i])
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

func NewHostLocator(value string) (*HostLocator, error) {
	if strings.Contains(value, "/") {
		return nil, errors.New("Host identifiers may not have a slash")
	}
	if value == "" || value == LocalHostName {
		return &HostLocator{}, nil
	}

	host, portString, err := net.SplitHostPort(value)
	if err != nil {
		return nil, err
	}
	id := &HostLocator{Host: host}
	if portString != "" {
		port, err := strconv.Atoi(portString)
		if err != nil {
			return nil, err
		}
		id.Port = containers.Port(port)
		if err := id.Port.Check(); err != nil {
			return nil, err
		}
	}
	return id, nil
}

func splitTypeHostId(value string) (res ResourceType, host string, id containers.Identifier, err error) {
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
		id, err = containers.NewIdentifier(sections[0])
		return
	}
	id, err = containers.NewIdentifier(sections[1])
	if err != nil {
		return
	}
	if strings.TrimSpace(sections[0]) == "" {
		err = errors.New("You must specify <host>/<id> or <id>")
		return
	}
	host = sections[0]
	return
}

func NewContainerLocators(values ...string) (Locators, error) {
	out := make(Locators, 0, len(values))
	for i := range values {
		r, err := NewContainerLocator(values[i])
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

func NewContainerLocator(value string) (*ContainerLocator, error) {
	res, hostString, id, errs := splitTypeHostId(value)
	if errs != nil {
		return nil, errs
	}
	if res != "" && ResourceType(res) != ResourceTypeContainer {
		return nil, errors.New(fmt.Sprintf("%s is not a container", value))
	}
	host, err := NewHostLocator(hostString)
	if err != nil {
		return nil, err
	}
	return &ContainerLocator{*host, id}, nil
}

func NewGenericLocators(defaultType ResourceType, values ...string) (Locators, error) {
	out := make(Locators, 0, len(values))
	for i := range values {
		r, err := NewGenericLocator(defaultType, values[i])
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

func NewGenericLocator(defaultType ResourceType, value string) (Locator, error) {
	res, hostString, id, errs := splitTypeHostId(value)
	if errs != nil {
		return nil, errs
	}
	if res == "" {
		res = defaultType
	}
	host, err := NewHostLocator(hostString)
	if err != nil {
		return nil, err
	}
	if ResourceType(res) == ResourceTypeContainer {
		return &ContainerLocator{*host, id}, nil
	}
	return &GenericLocator{*host, id, ResourceType(res)}, nil
}

func (r *HostLocator) IsDefaultPort() bool {
	return r.Port == 0
}
func (r *HostLocator) IsRemote() bool {
	return r.Host != ""
}
func (r *HostLocator) Identity() string {
	return r.HostIdentity()
}
func (r *HostLocator) HostIdentity() string {
	if r.Host != "" {
		if !r.IsDefaultPort() {
			return net.JoinHostPort(r.Host, strconv.Itoa(int(r.Port)))
		}
		return r.Host
	}
	return LocalHostName
}
func (r *HostLocator) ResolvedHostname() string {
	if r.Host != "" {
		return r.Host
	}
	return "localhost"
}
func (h *HostLocator) NewURI() (*url.URL, error) {
	port := "2223"
	if h.Port != containers.Port(0) {
		port = strconv.Itoa(int(h.Port))
	}
	return &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(h.ResolvedHostname(), port),
	}, nil
}
func (r *HostLocator) BaseURL() *url.URL {
	uri, err := r.NewURI()
	if err != nil {
		log.Fatal(err)
	}
	return uri
}
func (r *HostLocator) ResourceType() ResourceType {
	return ""
}

func (r *ContainerLocator) ResourceType() ResourceType {
	return ResourceTypeContainer
}
func (r *ContainerLocator) Identity() string {
	return r.HostLocator.HostIdentity() + "/" + string(r.Id)
}
func (r *ContainerLocator) Identifier() containers.Identifier {
	return r.Id
}

func (r *GenericLocator) ResourceType() ResourceType {
	return r.Type
}
func (r *GenericLocator) Identity() string {
	return string(r.Type) + "://" + r.HostLocator.HostIdentity() + "/" + string(r.Id)
}
func (r *GenericLocator) Identifier() containers.Identifier {
	return r.Id
}
