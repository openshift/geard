// +build idler

package cmd

import (
	"github.com/openshift/cobra"
	cmd "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/docker"
	"github.com/openshift/geard/idler"
	"github.com/openshift/geard/systemd"
)

var (
	hostIp       string
	idleTimeout  int
	dockerSocket string
)

func LoadCommand(gearCmd *cobra.Command, pDockerSocket *string, pHostIp *string) {
	idlerCmd := &cobra.Command{
		Use:   "idler-daemon",
		Short: "Idler is a daemon process for monitoring container traffic and idling/un-idling them",
		Run:   geardIdler,
	}
	idlerCmd.PersistentFlags().IntVarP(&idleTimeout, "idle-timeout", "T", 60, "Set the number of minutes of inactivity before an application is idled")
	dockerSocket = *pDockerSocket
	hostIp = *pHostIp

	gearCmd.AddCommand(idlerCmd)
}

func geardIdler(c *cobra.Command, args []string) {
	systemd.Require()

	dockerClient, err := docker.GetConnection(dockerSocket)
	if err != nil {
		cmd.Fail(1, "Unable to connect to docker on URI %v", dockerSocket)
	}

	if err := idler.StartIdler(dockerClient, hostIp, idleTimeout); err != nil {
		cmd.Fail(2, err.Error())
	}
}
