// +build idler

package cmd

import (
	cmd "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/docker"
	"github.com/openshift/geard/idler"
	"github.com/openshift/geard/systemd"
	"github.com/spf13/cobra"
	
	"net"
	"strings"
)

var (
	hostIp      string
	idleTimeout int
)

func init() {
	cmd.AddCommandExtension(func(parent *cobra.Command) {
		idlerCmd := &cobra.Command{
			Use:   "idler-daemon",
			Short: "(local) A daemon that monitors container traffic and makes idle/unidle decisions",
			Run:   startIdler,
		}
		idlerCmd.PersistentFlags().StringVarP(&hostIp, "host-ip", "H", guessHostIp(), "IP address to listen for traffic on")
		idlerCmd.PersistentFlags().IntVarP(&idleTimeout, "idle-timeout", "T", 60, "Set the number of minutes of inactivity before an application is idled")
		parent.AddCommand(idlerCmd)
	}, true)
}

func startIdler(c *cobra.Command, args []string) {
	systemd.Require()
	dockerSocket := c.Flags().Lookup("docker-socket").Value.String()

	dockerClient, err := docker.GetConnection(dockerSocket)
	if err != nil {
		cmd.Fail(1, "Unable to connect to docker on URI %s", dockerSocket)
	}

	if err := idler.StartIdler(dockerClient, hostIp, idleTimeout); err != nil {
		cmd.Fail(2, err.Error())
	}
}

func guessHostIp() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name, "veth") || strings.HasPrefix(iface.Name, "lo") ||
			strings.HasPrefix(iface.Name, "docker") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return ""
		}

		if len(addrs) == 0 {
			continue
		}

		ip, _, _ := net.ParseCIDR(addrs[0].String())
		return ip.String()
	}

	return ""
}
