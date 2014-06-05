package main

import (
	"github.com/spf13/cobra"
	"log"
	nethttp "net/http"
	"path/filepath"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/dispatcher"
	"github.com/openshift/geard/encrypted"
	"github.com/openshift/geard/http"
)

var (
	listenAddr string
	keyPath    string

	conf = http.HttpConfiguration{
		Dispatcher: &dispatcher.Dispatcher{
			QueueFast:         1000,
			QueueSlow:         100,
			Concurrent:        4,
			TrackDuplicateIds: 1000,
		},
	}
)

func registerHttpDaemonCommands(parent *cobra.Command) {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "(Local) Start the gear agent",
		Long:  "Launch the gear agent. Will not send itself to the background.",
		Run:   httpDaemon,
	}
	daemonCmd.Flags().StringVar(&keyPath, "key-path", "", "Specify the directory containing the server private key and trusted client public keys")
	daemonCmd.Flags().StringVarP(&listenAddr, "listen-address", "A", ":43273", "Set the address for the http endpoint to listen on")
	parent.AddCommand(daemonCmd)
}

func httpDaemon(c *cobra.Command, args []string) {
	api, err := conf.Handler()
	if err != nil {
		cmd.Fail(1, "Unable to start server: %s", err.Error())
	}
	nethttp.Handle("/", api)

	if keyPath != "" {
		config, err := encrypted.NewTokenConfiguration(filepath.Join(keyPath, "server"), filepath.Join(keyPath, "client.pub"))
		if err != nil {
			cmd.Fail(1, "Unable to load token configuration: %s", err.Error())
		}
		nethttp.Handle("/token/", nethttp.StripPrefix("/token", config.Handler(api)))
	}

	conf.Dispatcher.Start()

	log.Printf("Listening (HTTP) on %s ...", listenAddr)
	log.Fatal(nethttp.ListenAndServe(listenAddr, nil))
}
