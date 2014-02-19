package geard

import (
	"fmt"
	//switchns "github.com/kraman/geard-switchns/switchns"
	"log"
	"os/exec"
	"strconv"
)

type extendedLinkContainersData struct {
	LocalIP    string
	LocalPort  int
	RemoteIP   string
	RemotePort int
}

type linkContainersJobRequest struct {
	JobResponse
	jobRequest
	LocalGearId Identifier
	Data        extendedLinkContainersData
}

func executeCommandInContainer(containerName string, args []string) (string, error) {
	log.Printf("Executing %v in container %v\n", args, containerName)
	//switchns.JoinContainer(containerName, args, nil)
	cmdArgs := append([]string{containerName}, args...)
	out, err := exec.Command("/usr/local/bin/docker-exec.sh", cmdArgs...).Output()
	if err != nil {
		log.Printf("Failed to execute: %v\n", err)
		return "", err
	}
	log.Printf("Output: %v\n", string(out))
	return string(out), nil
}

func (j *linkContainersJobRequest) Execute() {
	containerName := fmt.Sprintf("gear-%v", j.LocalGearId)
	log.Println(containerName)
	cmdArgs := []string{"SNAT"}
	ipaddr, err := executeCommandInContainer(containerName, cmdArgs)
	if err != nil {
		return
	}
	cmdArgs = []string{"iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-j", "SNAT", "--to-source", ipaddr}
	_, err = executeCommandInContainer(containerName, cmdArgs)
	if err != nil {
		return
	}
	cmdArgs = []string{"iptables", "-t", "nat", "-L"}
	_, err = executeCommandInContainer(containerName, cmdArgs)
	if err != nil {
		return
	}
	dest := fmt.Sprintf("%v:%v", j.Data.RemoteIP, j.Data.RemotePort)
	cmdArgs = []string{"iptables", "-t", "nat", "-A", "PREROUTING", "-d", j.Data.LocalIP, "-m", "tcp", "-p", "tcp", "--dport", strconv.Itoa(j.Data.LocalPort), "-j", "DNAT", "--to-destination", dest}
	_, err = executeCommandInContainer(containerName, cmdArgs)
	if err != nil {
		return
	}
	cmdArgs = []string{"iptables", "-t", "nat", "-A", "OUTPUT", "-d", j.Data.LocalIP, "-m", "tcp", "-p", "tcp", "--dport", strconv.Itoa(j.Data.LocalPort), "-j", "DNAT", "--to-destination", dest}
	_, err = executeCommandInContainer(containerName, cmdArgs)
	if err != nil {
		return
	}
}
