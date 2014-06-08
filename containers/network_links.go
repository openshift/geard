package containers

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/openshift/geard/port"
)

type NetworkLink struct {
	FromHost string
	FromPort port.Port
	ToPort   port.Port `json:"ToPort,omitempty"`
	ToHost   string    `json:"ToHost,omitempty"`
}

type NetworkLinks []NetworkLink

func (n *NetworkLink) Check() error {
	if err := n.FromPort.Check(); err != nil {
		return errors.New("The from port value must be a positive integer less than 65536")
	}
	if !n.ToPort.Default() {
		if err := n.ToPort.Check(); err != nil {
			return errors.New("The to port value must be a positive integer less than 65536 or zero")
		}
	}
	return nil
}

func (n *NetworkLink) Complete() bool {
	return n.ToPort >= 1 && n.ToHost != ""
}

func (n NetworkLinks) Check() error {
	for i := range n {
		if err := n[i].Check(); err != nil {
			return err
		}
	}
	return nil
}

func (n NetworkLinks) Write(path string, appends bool) error {
	var file *os.File
	var err error

	if appends {
		file, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0660)
	} else {
		file, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0660)
		if os.IsExist(err) {
			file, err = os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0660)
		}
	}
	if err != nil {
		log.Print("network_links: Unable to open network links file: ", err)
		return err
	}
	defer file.Close()

	for i := range n {
		if _, errw := fmt.Fprintf(file, "%s\t%d\t%d\t%s\n", n[i].FromHost, n[i].FromPort, n[i].ToPort, n[i].ToHost); errw != nil {
			log.Print("network_links: Unable to write network links: ", err)
			return err
		}
	}
	if errc := file.Close(); errc != nil {
		log.Print("network_links: Unable to network links: ", errc)
		return err
	}
	return nil
}

func (n NetworkLinks) String() string {
	var pairs bytes.Buffer
	for i := range n {
		if i != 0 {
			pairs.WriteString(", ")
		}
		pairs.WriteString(n[i].FromHost)
		pairs.WriteString(":")
		pairs.WriteString(strconv.Itoa(int(n[i].FromPort)))
		pairs.WriteString(" -> ")
		pairs.WriteString(n[i].ToHost)
		pairs.WriteString(":")
		pairs.WriteString(strconv.Itoa(int(n[i].ToPort)))
	}
	return pairs.String()
}

func NewNetworkLinksFromString(s string) (NetworkLinks, error) {
	set := strings.Split(s, ",")
	links := make(NetworkLinks, 0, len(set))
	for i := range set {
		link, err := NewNetworkLinkFromString(set[i])
		if err != nil {
			return NetworkLinks{}, err
		}
		links = append(links, *link)
	}
	return links, nil
}

func NewNetworkLinkFromString(s string) (*NetworkLink, error) {
	value := strings.Split(s, ":")
	if len(value) < 3 {
		return nil, errors.New(fmt.Sprintf("The network link '%s' must be of the form <from_host>:<from_port>:<to_host>:<to_port> where <from_host> is optional", s))
	}

	// Handle the case where from_host isn't specified
	if len(value) == 3 {
		value = append([]string{"127.0.0.1"}, value...)
	}

	link := NetworkLink{}
	link.FromHost = value[0]
	from_port, err := strconv.Atoi(value[1])
	if err != nil {
		return nil, err
	}
	link.FromPort = port.Port(from_port)
	if err := link.FromPort.Check(); err != nil {
		return nil, errors.New("From port value must be between 0 and 65535")
	}
	link.ToHost = value[2]
	if value[3] != "" {
		to, err := strconv.Atoi(value[3])
		if err != nil {
			return nil, err
		}
		link.ToPort = port.Port(to)
		if err := link.ToPort.Check(); err != nil {
			return nil, errors.New("To port value must be between 0 and 65535")
		}
	}
	return &link, nil
}

func (n NetworkLinks) ToCompact() string {
	var pairs bytes.Buffer
	for i := range n {
		if i != 0 {
			pairs.WriteString(",")
		}
		pairs.WriteString(n[i].FromHost)
		pairs.WriteString(":")
		pairs.WriteString(strconv.Itoa(int(n[i].FromPort)))
		pairs.WriteString(":")
		pairs.WriteString(n[i].ToHost)
		pairs.WriteString(":")
		pairs.WriteString(strconv.Itoa(int(n[i].ToPort)))
	}
	return pairs.String()
}

type ContainerLink struct {
	Id           Identifier
	NetworkLinks NetworkLinks
}

func (link *ContainerLink) Check() error {
	if link.Id == "" {
		return errors.New("Container identifier may not be empty")
	}
	if _, err := NewIdentifier(string(link.Id)); err != nil {
		return err
	}
	for i := range link.NetworkLinks {
		if err := link.NetworkLinks[i].Check(); err != nil {
			return err
		}
	}
	return nil
}

type ContainerLinks struct {
	Links []ContainerLink
}

func (link *ContainerLinks) Check() error {
	if len(link.Links) == 0 {
		return errors.New("One or more links must be specified.")
	}
	for i := range link.Links {
		if err := link.Links[i].Check(); err != nil {
			return err
		}
	}
	return nil
}

func (link *ContainerLinks) String() string {
	buf := bytes.Buffer{}
	for i := range link.Links {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(string(link.Links[i].Id))
	}
	return buf.String()
}
