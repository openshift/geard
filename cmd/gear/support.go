// TODO: Needs to be folded into an execution driver or a 'gear run'
// command.
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/docker"
	"github.com/openshift/geard/ssh"
	"github.com/openshift/geard/utils"

	dc "github.com/fsouza/go-dockerclient"
)

func InitPreStart(dockerSocket string, id containers.Identifier, imageName string) error {
	if config.SystemDockerFeatures.EnvironmentFile {
		return containers.UpdateInitEnvironment(dockerSocket, id, imageName)
	} else {
		return containers.CreateContainerInitScripts(dockerSocket, id, imageName)
	}
}

func InitPostStart(dockerSocket string, id containers.Identifier) error {
	var (
		u         *user.User
		container *dc.Container
		err       error
		d         *docker.DockerClient
	)

	if u, err = user.Lookup(id.LoginFor()); err == nil {
		if err := ssh.GenerateAuthorizedKeysFor(u, true, false); err != nil {
			log.Print(err.Error())
		}
	} else {
		log.Print(err.Error())
	}

	if d, err = docker.GetConnection(dockerSocket); err != nil {
		return err
	}

	if file, err := os.Open(id.NetworkLinksPathFor()); err == nil {
		defer file.Close()

		const ContainerInterval = time.Second / 3
		const ContainerWait = time.Second * 3
		for i := 0; i < int(ContainerWait/ContainerInterval); i++ {
			if container, err = d.GetContainer(id.ContainerFor(), true); err != nil {
				return err
			}
			if container.State.Running {
				break
			} else {
				log.Printf("Waiting for container to run.")
				time.Sleep(ContainerInterval)
			}
		}

		pid, err := d.ChildProcessForContainer(container)
		if err != nil {
			return err
		}
		if pid < 2 {
			return errors.New("support: child PID is not correct")
		}
		log.Printf("Updating network namespaces for %d", pid)
		if err := updateNamespaceNetworkLinks(pid, file); err != nil {
			return err
		}
	}

	return nil
}

func getHostIPFromNamespace(name string) (*net.IPAddr, error) {
	// Resolve the containers local IP
	cmd := exec.Command("ip", "netns", "exec", name, "hostname", "-I")
	cmd.Stderr = os.Stderr
	source, erro := cmd.Output()
	if erro != nil {
		log.Printf("gear: Could not read IP for container: %v", erro)
		return nil, erro
	}
	sourceAddr, errr := net.ResolveIPAddr("ip", strings.TrimSpace(string(source)))
	if errr != nil {
		log.Printf("gear: Host source IP %s does not resolve %v", sourceAddr, errr)
		return nil, errr
	}
	return sourceAddr, nil
}

func updateNamespaceNetworkLinks(pid int, ports io.Reader) error {
	name := "netlink-" + strconv.Itoa(pid)
	nsPath := fmt.Sprintf("/proc/%d/ns/net", pid)
	path := fmt.Sprintf("/var/run/netns/%s", name)

	if err := os.MkdirAll("/var/run/netns", 0755); err != nil {
		return err
	}

	if err := os.Symlink(nsPath, path); err != nil && !os.IsExist(err) {
		return err
	}
	defer os.Remove(path)

	sourceAddr, errs := getHostIPFromNamespace(name)
	if errs != nil {
		return errs
	}

	// Enable routing in the namespace
	output, err := exec.Command("ip", "netns", "exec", name, "sysctl", "-w", "net.ipv4.conf.all.route_localnet=1").Output()
	if err != nil {
		log.Printf("gear: Failed to enable localnet routing: %v", err)
		log.Printf("gear: error output: %v", output)
		return err
	}

	// Enable ip forwarding
	output, err = exec.Command("ip", "netns", "exec", name, "sysctl", "-w", "net.ipv4.ip_forward=1").Output()
	if err != nil {
		log.Printf("gear: Failed to enable ipv4 forwarding: %v", err)
		log.Printf("gear: error output: %v", output)
		return err
	}

	// Restore a set of rules to the table
	cmd := exec.Command("ip", "netns", "exec", name, "iptables-restore")
	stdin, errp := cmd.StdinPipe()
	if errp != nil {
		log.Printf("gear: Could not open pipe to iptables-restore: %v", errp)
		return errp
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		log.Printf("gear: Could not start iptables-restore: %v", errp)
		return err
	}

	fmt.Fprintf(stdin, "*nat\n")
	for {
		link := containers.NetworkLink{}
		if _, err := fmt.Fscanf(ports, "%s\t%v\t%v\t%s\n", &link.FromHost, &link.FromPort, &link.ToPort, &link.ToHost); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("gear: Could not read from network links file: %v", err)
			continue
		}
		if err := link.Check(); err != nil {
			log.Printf("gear: Link in file is not valid: %v", err)
			continue
		}
		if link.Complete() {
			srcIP, err := net.ResolveIPAddr("ip", link.FromHost)
			if err != nil {
				log.Printf("gear: Link source host does not resolve %v", err)
				continue
			}

			destIP, err := utils.Resolver.ResolveIP(link.ToHost)
			if err != nil {
				log.Printf("gear: Link destination host does not resolve %v", err)
				continue
			}

			data := containers.OutboundNetworkIptables{sourceAddr.String(), srcIP.IP.String(), link.FromPort, destIP.String(), link.ToPort}
			if err := containers.OutboundNetworkIptablesTemplate.Execute(stdin, &data); err != nil {
				log.Printf("gear: Unable to write network link rules: %v", err)
				return err
			}
		}
	}
	fmt.Fprintf(stdin, "COMMIT\n")

	stdin.Close()
	if err := cmd.Wait(); err != nil {
		log.Printf("gear: iptables-restore did not successfully complete: %v", err)
		return err
	}
	return nil
}
