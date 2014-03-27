// +build idler

package iptables

import (
	gearconfig "github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/docker"
	"github.com/openshift/geard/idler/config"

	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	localhost = "127.0.0.1"
)

func runIptablesRules(add bool, shouldQueue bool, hostIp string, port string, id containers.Identifier) error {
	command := []string{"/sbin/iptables"}
	if config.UsePreroutingIdler {
		var chain string
		var table string

		if hostIp == localhost {
			chain = "OUTPUT"
			table = "raw"
		} else {
			chain = "PREROUTING"
			table = "nat"
		}

		if add {
			command = append(command, "-t", table, "-I", chain, "1")
		} else {
			command = append(command, "-t", table, "-D", chain)
		}
	} else {
		if add {
			command = append(command, "-I", "INPUT", "1")
		} else {
			command = append(command, "-D", "INPUT")
		}
	}

	command = append(command, "-d", hostIp, "-p", "tcp", "-m", "tcp", "--dport", port)
	if shouldQueue {
		command = append(command, "-j", "NFQUEUE", "--queue-num", "0")
	} else {
		command = append(command, "-j", "ACCEPT")
	}
	command = append(command, "-m", "comment", "--comment", string(id))
	return exec.Command(command[0], command[1:]...).Run()
}

func IdleContainer(id containers.Identifier, hostIp string) {
	portPairs, err := containers.GetExistingPorts(id)
	if err != nil {
		fmt.Printf("IdleContainer: Error retrieving ports for container: %v\n", id)
		return
	}

	for _, portPair := range portPairs {
		port := portPair.External
		runIptablesRules(false, true, hostIp, port.String(), id)
		runIptablesRules(false, true, localhost, port.String(), id)

		runIptablesRules(true, true, hostIp, port.String(), id)
		runIptablesRules(true, true, localhost, port.String(), id)

		runIptablesRules(false, false, localhost, port.String(), id)
	}
}

func UnidleContainer(id containers.Identifier, hostIp string) {
	portPairs, err := containers.GetExistingPorts(id)
	if err != nil {
		fmt.Printf("UnidleContainer: Error retrieving ports for container: %v\n", id)
		return
	}

	for _, portPair := range portPairs {
		port := portPair.External
		runIptablesRules(false, true, hostIp, port.String(), id)
		runIptablesRules(false, true, localhost, port.String(), id)

		runIptablesRules(false, false, localhost, port.String(), id)
		runIptablesRules(true, false, localhost, port.String(), id)
	}
}

func DeleteContainer(id containers.Identifier, hostIp string) {
	portMap, err := GetIdlerRules(id, true)
	if err != nil {
		fmt.Printf("DeleteContainer: Error retrieving ports for container: %v\n", id)
		return
	}

	for port := range portMap {
		runIptablesRules(false, true, hostIp, port, id)
		runIptablesRules(false, true, localhost, port, id)
		runIptablesRules(false, true, hostIp, port, id)
		runIptablesRules(false, true, localhost, port, id)
		runIptablesRules(false, false, localhost, port, id)
	}

	portMap, err = GetIdlerRules(id, false)
	if err != nil {
		fmt.Printf("DeleteContainer: Error retrieving ports for container: %v\n", id)
		return
	}

	for port := range portMap {
		runIptablesRules(false, true, hostIp, port, id)
		runIptablesRules(false, true, localhost, port, id)
		runIptablesRules(false, true, hostIp, port, id)
		runIptablesRules(false, true, localhost, port, id)
		runIptablesRules(false, false, localhost, port, id)
	}
}

func GetDockerContainerPacketCounts(d *docker.DockerClient) (map[containers.Identifier]int, error) {
	serviceFiles, err := filepath.Glob(filepath.Join(gearconfig.ContainerBasePath(), "units", "**", containers.IdentifierPrefix+"*.service"))
	if err != nil {
		return nil, err
	}

	ids := make([]containers.Identifier, 0)
	packetCount := make(map[containers.Identifier]int)

	for _, s := range serviceFiles {
		id := filepath.Base(s)
		if strings.HasPrefix(id, containers.IdentifierPrefix) && strings.HasSuffix(id, ".service") {
			id = id[len(containers.IdentifierPrefix):(len(id) - len(".service"))]
			if id, err := containers.NewIdentifier(id); err == nil {
				ids = append(ids, id)
				packetCount[id] = 0
			}
		}
	}

	containerIPs, err := containers.GetContainerIPs(d, ids)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("/sbin/iptables-save", "-c")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	scan := bufio.NewScanner(bytes.NewBuffer(output))
	for scan.Scan() {
		line := scan.Text()
		if strings.Contains(line, "-A DOCKER ! -i docker0") && strings.Contains(line, "-j DNAT") {
			//Example: [0:0] -A DOCKER ! -i docker0 -p tcp -m tcp --dport 4000 -j DNAT --to-destination 172.17.0.3:8080
			items := strings.Fields(line)
			packets, _ := strconv.Atoi(strings.Split(items[0], ":")[0][1:])
			destIp := strings.Split(items[15], ":")[0]
			id := containerIPs[destIp]

			packetCount[id] = packetCount[id] + packets
		}

		if strings.Contains(line, "-A OUTPUT -d 127.0.0.1/32 -p tcp -m tcp --dport") && strings.Contains(line, "-m comment --comment ") {
			//Example: [5850:394136] -A OUTPUT -d 127.0.0.1/32 -p tcp -m tcp --dport 4000 -m comment --comment 0001 -j ACCEPT
			items := strings.Fields(line)
			packets, _ := strconv.Atoi(strings.Split(items[0], ":")[0][1:])
			if id, err := containers.NewIdentifier(items[14]); err == nil {
				packetCount[id] = packetCount[id] + packets
			}
		}
	}

	return packetCount, nil
}

func GetIdlerRules(lookupId containers.Identifier, active bool) (map[string]bool, error) {
	cmd := exec.Command("/sbin/iptables-save", "-c")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	scan := bufio.NewScanner(bytes.NewBuffer(output))
	ports := make(map[string]bool)

	for scan.Scan() {
		line := scan.Text()
		items := strings.Fields(line)

		var (
			port string
			id   string
		)

		if config.UsePreroutingIdler {
			if !active && strings.Contains(line, "-A PREROUTING") && strings.Contains(line, "-j NFQUEUE --queue-num 0") {
				port = items[10]
				id = items[14]
			} else if active && strings.Contains(line, "-A OUTPUT") && strings.Contains(line, "-j ACCEPT") && strings.Contains(line, "-m comment --comment ") {
				port = items[10]
				id = items[14]
			} else {
				continue
			}
		} else {
			if active && strings.Contains(line, "-A INPUT") && strings.Contains(line, "-j NFQUEUE --queue-num 0") {
				port = items[10]
				id = items[14]
			} else {
				continue
			}
		}

		ruleId, err := containers.NewIdentifier(id)
		if err != nil {
			return nil, err
		}

		if ruleId != lookupId {
			continue
		}

		ports[port] = true
	}
	return ports, nil
}

func ResetPacketCount() error {
	err := exec.Command("/sbin/iptables", "-t", "nat", "-L", "DOCKER", "-Z").Run()
	if err != nil {
		return err
	}
	return exec.Command("/sbin/iptables", "-t", "raw", "-Z").Run()
}

func CleanupRulesForPort(p containers.Port) {
	fmt.Printf("Cleaning stale rules for port %v\n", p)
	port := p.String()
	cmd := exec.Command("/sbin/iptables-save", "-c")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	scan := bufio.NewScanner(bytes.NewBuffer(output))
	for scan.Scan() {
		line := scan.Text()
		items := strings.Fields(line)

		if config.UsePreroutingIdler {
			if (strings.Contains(line, "-A PREROUTING") && strings.Contains(line, "-j NFQUEUE --queue-num 0") && port == items[10]) ||
				(strings.Contains(line, "-A OUTPUT") && strings.Contains(line, "-j ACCEPT") && strings.Contains(line, "-m comment --comment ") && port == items[10]) ||
				(strings.Contains(line, "-A OUTPUT") && strings.Contains(line, "-j NFQUEUE --queue-num 0") && port == items[10]) {

				id, err := containers.NewIdentifier(items[14])
				if err != nil {
					return
				}
				hostIp := items[4]

				if hostIp == "127.0.0.1/32" {
					runIptablesRules(false, true, localhost, port, id)
					runIptablesRules(false, true, localhost, port, id)
					runIptablesRules(false, false, localhost, port, id)
				} else {
					runIptablesRules(false, true, hostIp, port, id)
					runIptablesRules(false, true, hostIp, port, id)
				}
			}
		}
	}
}
