package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/daemon"
	"github.com/openshift/geard/dispatcher"
)

var (
	dispatch = &dispatcher.Dispatcher{
		QueueFast:         1000,
		QueueSlow:         100,
		Concurrent:        4,
		TrackDuplicateIds: 1000,
	}
)

type commandExtension interface {
	RegisterCommand(parent *cobra.Command)
	ExampleUrls() []string
}

type Command struct {
	DefaultAddr string
}

func (d *Command) RegisterLocal(parent *cobra.Command) {
	daemonCmd := &cobra.Command{
		Use:   "daemon <on>...",
		Short: "(Local) Start the gear agent",
		Long:  fmt.Sprintf("Launch the gear agent. Will not send itself to the background.\n\nSpecify one or more addresses to listen on. The default address is %s.", d.DefaultAddr),
		Run:   d.startDaemon,
	}
	examples := []string{}
	for _, ext := range daemon.DaemonExtensions() {
		if cmdExt, ok := ext.(commandExtension); ok {
			cmdExt.RegisterCommand(daemonCmd)
			examples = append(examples, cmdExt.ExampleUrls()...)
		}
	}
	daemonCmd.Long += fmt.Sprintf("\n\nValid address types:\n  %s", strings.Join(examples, "  \n"))
	parent.AddCommand(daemonCmd)
}

func (d *Command) startDaemon(c *cobra.Command, args []string) {
	if len(args) == 0 {
		args = []string{d.DefaultAddr}
	}

	exts := []daemon.DaemonExtension{}
	for i := range args {
		ext, err := daemon.DaemonExtensionFor(args[i])
		if err != nil {
			cmd.Fail(1, "Can't start server: %s", err.Error())
		}
		exts = append(exts, ext)
	}

	dispatch.Start()

	done := make(chan bool)
	errs := make(chan error, len(args))
	wg := sync.WaitGroup{}
	for i := range args {
		ext := exts[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ext.Start(args[i], dispatch, done); err != nil {
				errs <- err
				log.Printf("server[%s] encountered error: %s", args[i], err.Error())
			}
		}()
	}
	wg.Wait()
	select {
	case <-errs:
		os.Exit(1)
	default:
	}
}
