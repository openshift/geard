package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	nethttp "net/http"
	"net/url"
	"path/filepath"

	"github.com/openshift/geard/dispatcher"
	"github.com/openshift/geard/encrypted"
	"github.com/openshift/geard/http"
)

type Daemon struct {
	keyPath string
}

func (d *Daemon) RegisterCommand(parent *cobra.Command) {
	parent.Flags().StringVar(&d.keyPath, "key-path", "", "Specify the directory containing the server private key and trusted client public keys")
}

func (d *Daemon) ExampleUrls() []string {
	return []string{"HTTP: http://<bind-ip>[:port]"}
}

func (d *Daemon) Schemes() []string {
	return []string{"http"}
}

func (d *Daemon) Start(addr string, dispatch *dispatcher.Dispatcher, done chan<- bool) error {
	conf := http.HttpConfiguration{Dispatcher: dispatch}

	addrUrl, err := url.Parse(addr)
	if err != nil {
		return err
	}
	hostPort := addrUrl.Host

	api, err := conf.Handler()
	if err != nil {
		return err
	}

	if d.keyPath != "" {
		config, err := encrypted.NewTokenConfiguration(filepath.Join(d.keyPath, "server"), filepath.Join(d.keyPath, "client.pub"))
		if err != nil {
			return fmt.Errorf("unable to load token configuration: %s", err.Error())
		}
		nethttp.Handle("/token/", nethttp.StripPrefix("/token", config.Handler(api)))
	}

	nethttp.Handle("/", api)

	log.Printf("Listening (HTTP) on %s ...", hostPort)
	return nethttp.ListenAndServe(hostPort, nil)
}
