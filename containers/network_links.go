package containers

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type NetworkLink struct {
	FromPort Port
	ToPort   Port   `json:"ToPort,omitempty"`
	ToHost   string `json:"ToHost,omitempty"`
}

type NetworkLinks []NetworkLink

func (n *NetworkLink) Check() error {
	if err := n.FromPort.Check(); err != nil {
		return errors.New("The from port value must be a positive integer less than 65536")
	}
	if err := n.ToPort.CheckDefault(); err != nil {
		return errors.New("The to port value must be a positive integer less than 65536 or zero")
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
		if _, errw := fmt.Fprintf(file, "%d\t%d\t%s\n", n[i].FromPort, n[i].ToPort, n[i].ToHost); errw != nil {
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
	value := strings.SplitN(s, ":", 3)
	if len(value) != 3 {
		return nil, errors.New(fmt.Sprintf("The network link '%s' must be of the form <from>:<to_host>:<to_port>", s))
	}
	link := NetworkLink{}
	from, err := strconv.Atoi(value[0])
	if err != nil {
		return nil, err
	}
	link.FromPort = Port(from)
	if err := link.FromPort.Check(); err != nil {
		return nil, errors.New("From port value must be between 0 and 65535")
	}
	link.ToHost = value[1]
	if value[2] != "" {
		to, err := strconv.Atoi(value[2])
		if err != nil {
			return nil, err
		}
		link.ToPort = Port(to)
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
		pairs.WriteString(strconv.Itoa(int(n[i].FromPort)))
		pairs.WriteString(":")
		pairs.WriteString(n[i].ToHost)
		pairs.WriteString(":")
		pairs.WriteString(strconv.Itoa(int(n[i].ToPort)))
	}
	return pairs.String()
}
