// An executable that can run as an agent for remote execution,
// make calls to a remote agent, or invoke commands locally.
package main

import (
	"fmt"
	"log"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/config"
	"github.com/spf13/cobra"
)

var (
	version string

	defaultTransport LocalTransportFlag

	insecure bool
)

func init() {
	log.SetFlags(0)
}

func main() {
	gearCmd := &cobra.Command{
		Use:   "gear",
		Long: "Gear(d) is a tool for installing Docker containers to systemd.\n\n"+
		      "A command line client and server that installs Docker containers as systemd units.\n"+
		      "Complete documentation is available at http://github.com/openshift/geard",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}
	gearCmd.PersistentFlags().Var(&defaultTransport, "transport", "Specify an alternate mechanism to connect to the gear agent")
	gearCmd.PersistentFlags().BoolVar(&(config.SystemDockerFeatures.EnvironmentFile), "has-env-file", true, "Use --env-file with Docker, set false if older than 0.11")
	gearCmd.PersistentFlags().BoolVar(&(config.SystemDockerFeatures.ForegroundRun), "has-foreground", false, "(experimental) Use --foreground with Docker, requires alexlarsson/forking-run")
	gearCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "k", false, "Do not verify CA certificate on SSL connections and transfers")

	// declare remote, then local commands
	cmd.ExtendCommands(gearCmd, false)
	cmd.ExtendCommands(gearCmd, true)

	// version information
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version",
		//Long:  "Display version",
		Run: func(c *cobra.Command, args []string) {
			fmt.Printf("gear %s\n", version)
		},
	}
	gearCmd.AddCommand(versionCmd)

	// run
	if err := gearCmd.Execute(); err != nil {
		cmd.Fail(1, err.Error())
	}
}
