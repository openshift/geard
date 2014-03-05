package containers

import (
	"errors"
	"fmt"
	"log"
	"os"
)

type NetworkLink struct {
	FromPort Port   `json:"from_port"`
	ToPort   Port   `json:"to_port"`
	ToHost   string `json:"to_host"`
}

type NetworkLinks []NetworkLink

func (n *NetworkLink) Check() error {
	if n.FromPort < 1 || n.FromPort > 65535 {
		return errors.New("The from port value must be a positive integer less than 65536")
	}
	if n.ToPort > 65535 {
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
