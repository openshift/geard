package main

import (
	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/encrypted"
	"github.com/openshift/geard/systemd"

	"github.com/spf13/cobra"
	"log"
	nethttp "net/http"
	"path/filepath"
)

func daemon(cmd *cobra.Command, args []string) {
	api := conf.Handler()
	nethttp.Handle("/", api)

	if keyPath != "" {
		config, err := encrypted.NewTokenConfiguration(filepath.Join(keyPath, "server"), filepath.Join(keyPath, "client.pub"))
		if err != nil {
			Fail(1, "Unable to load token configuration: %s", err.Error())
		}
		nethttp.Handle("/token/", nethttp.StripPrefix("/token", config.Handler(api)))
	}

	if err := systemd.Start(); err != nil {
		log.Fatal(err)
	}
	if err := containers.InitializeData(); err != nil {
		log.Fatal(err)
	}
	if err := Initialize(ForDaemon); err != nil {
		log.Fatal(err)
	}

	conf.Dispatcher.Start()

	log.Printf("Listening (HTTP) on %s ...", listenAddr)
	log.Fatal(nethttp.ListenAndServe(listenAddr, nil))
}
