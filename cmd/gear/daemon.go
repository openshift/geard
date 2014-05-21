package main

import (
	"github.com/spf13/cobra"
	"log"
	nethttp "net/http"
	// "path/filepath"

	"github.com/openshift/geard/cmd"
	// "github.com/openshift/geard/encrypted"
)

func daemon(c *cobra.Command, args []string) {
	api, err := conf.Handler()
	if err != nil {
		cmd.Fail(1, "Unable to start server: %s", err.Error())
	}
	nethttp.Handle("/", api)

	// if keyPath != "" {
	// 	config, err := encrypted.NewTokenConfiguration(filepath.Join(keyPath, "server"), filepath.Join(keyPath, "client.pub"))
	// 	if err != nil {
	// 		cmd.Fail(1, "Unable to load token configuration: %s", err.Error())
	// 	}
	// 	nethttp.Handle("/token/", nethttp.StripPrefix("/token", config.Handler(api)))
	// }

	conf.Dispatcher.Start()

	log.Printf("Listening (HTTP) on %s ...", listenAddr)
	log.Fatal(nethttp.ListenAndServe(listenAddr, nil))
}
