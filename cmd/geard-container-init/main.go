package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"io"
	"net"

	"github.com/openshift/geard/containers"
)

func findCommandOrExit(cmd string) (path string) {
	var err error
	if path, err = exec.LookPath(cmd); err != nil {
		log.Printf("%v not found in path", cmd)
		os.Exit(1)
	}
	return
}

func runCommandOrExit(cmd []string) (out string) {
	var err error
	var data []byte

	if data, err = exec.Command(cmd[0], cmd[1:]...).Output(); err != nil {
		log.Printf("Unable to run command %v: %v", cmd, err)
		os.Exit(1)
	}

	out = string(data)
	log.Printf("%v => %v", cmd, out)
	return
}

func main() {
	var (
		data             *containers.ContainerData
		shouldCreateUser bool
		userName         string
		err              error
	)

	container_info := os.Getenv("GEARD_CONTAINER_INFO")
	if container_info == "" {
		log.Println("No container information provided. Exiting")
		os.Exit(1)
	}

	if container_info, err = strconv.Unquote(container_info); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	data = &containers.ContainerData{}
	if err = json.Unmarshal([]byte(container_info), data); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	userName = data.User
	if data.User == "" {
		shouldCreateUser = true
		userName = "container"
	}

	useraddCmd := findCommandOrExit("useradd")
	groupaddCmd := findCommandOrExit("groupadd")
	idCmd := findCommandOrExit("id")
	usermodCmd := findCommandOrExit("usermod")
	groupmodCmd := findCommandOrExit("groupmod")
	bashCmd := findCommandOrExit("bash")
	chownCmd := findCommandOrExit("chown")
	chgrpCmd := findCommandOrExit("chgrp")
	suCmd := findCommandOrExit("su")

	if shouldCreateUser {
		runCommandOrExit([]string{groupaddCmd, "-g", data.Gid, userName})
		runCommandOrExit([]string{useraddCmd, "-u", data.Uid, "-g", data.Gid, userName})
	} else {
		oldUid := runCommandOrExit([]string{idCmd, "-u", userName})
		oldGid := runCommandOrExit([]string{idCmd, "-g", userName})
		runCommandOrExit([]string{usermodCmd, "--uid", data.Uid, userName})
		runCommandOrExit([]string{groupmodCmd, "--uid", data.Uid, userName})

		cmd := fmt.Sprintf("for i in $(find / -uid %v); do %v -R %v $i; done", oldUid, chownCmd, data.Uid)
		runCommandOrExit([]string{bashCmd, "-c", cmd})

		cmd = fmt.Sprintf("for i in $(find / -gid %v); do %v -R %v $i; done", oldGid, chgrpCmd, data.Gid)
		runCommandOrExit([]string{bashCmd, "-c", cmd})
	}

	if data.Volumes != nil && len(data.Volumes) > 0 {
		volumes := strings.Join(data.Volumes, " ")
		runCommandOrExit([]string{chownCmd, "-R", data.Uid + ":" + data.Gid, volumes})
	}

	if data.SocketActivated {
		cmd := "LISTEN_PID=$$ exec /usr/sbin/systemd-socket-proxyd"
		for _, p := range data.Ports {
			cmd = cmd + "127.0.0.1:" + p.Internal.String()
		}
		cmd = cmd + " &"

		runCommandOrExit([]string{bashCmd, "-c", cmd})
	}

	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "GEARD_CONTAINER_INFO=") {
			env = append(env[:i], env[(i+1):]...)
			break
		}
	}

	processLinks(data.Links)

	syscall.Exec(suCmd, []string{suCmd, userName, "-s", bashCmd, "-c", strings.Join(data.Command, " ")}, env)
}

func getIPAddress() (ip string, err error) {
	var (
		ifaces []net.Interface
		addrs  []net.Addr
		ipAddr net.IP
	)

	ifaces, err = net.Interfaces()
	for _, iface := range ifaces {
		if iface.Name != "lo" {
			if addrs, err = iface.Addrs(); err != nil {
				return
			}
			for _, addr := range addrs {
				if ipAddr,_, err = net.ParseCIDR(addr.String()); err != nil {
					continue
				}
				ip = ipAddr.String()
				log.Printf("Local IP is %v", ip)
				return
			}
		}
	}

	err = fmt.Errorf("No external IPs found")
	return
}

func processLinks(links []containers.NetworkLink) (err error) {
	var (
		stdin      io.WriteCloser
		sourceAddr string
		srcIP      *net.IPAddr
	)

	if sourceAddr, err = getIPAddress(); err != nil {
		return
	}

	sysctlCmd := findCommandOrExit("sysctl")
	iptablesRestoreCmd := findCommandOrExit("iptables-restore")

	runCommandOrExit([]string{sysctlCmd, "-w", "net.ipv4.conf.all.route_localnet=1", "-w", "net.ipv4.ip_forward=1"})

	// Restore a set of rules to the table
	cmd := exec.Command(iptablesRestoreCmd)
	stdin, err = cmd.StdinPipe()
	if err != nil {
		log.Printf("gear: Could not open pipe to iptables-restore: %v", err)
		return
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	defer stdin.Close()

	if err = cmd.Start(); err != nil {
		log.Printf("Could not start iptables-restore: %v", err)
		return
	}

	fmt.Fprintf(stdin, "*nat\n")
	for _, link := range links {
		if err = link.Check(); err != nil {
			log.Printf("Link in file is not valid: %v", err)
			continue
		}

		if link.Complete() {
			srcIP, err = net.ResolveIPAddr("ip", link.FromHost)
			if err != nil {
				log.Printf("gear: Link source host does not resolve %v", err)
				continue
			}

			data := containers.OutboundNetworkIptables{sourceAddr, srcIP.IP.String(), link.FromPort, link.ToHost, link.ToPort}

			if err = containers.OutboundNetworkIptablesTemplate.Execute(stdin, &data); err != nil {
				log.Printf("gear: Unable to write network link rules: %v", err)
				return
			}
		}
	}
	fmt.Fprintf(stdin, "COMMIT\n")

	stdin.Close()
	if err = cmd.Wait(); err != nil {
		log.Printf("gear: iptables-restore did not successfully complete: %v", err)
		return
	}

	return
}
