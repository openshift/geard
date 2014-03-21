package containers

import (
	"bufio"
	"errors"
	"fmt"
	dc "github.com/fsouza/go-dockerclient"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/docker"
	"github.com/smarterclayton/geard/selinux"
	"github.com/smarterclayton/geard/utils"
)

var resolver addressResolver = addressResolver{}

func InitPreStart(dockerSocket string, id Identifier, imageName string) error {
	var (
		err     error
		imgInfo *dc.Image
		d       *docker.DockerClient
	)

	_, socketActivationType, err := GetSocketActivation(id)
	if err != nil {
		fmt.Printf("init_pre_start: Error while parsing unit file: %v\n", err)
		return err
	}

	if _, err = user.Lookup(id.LoginFor()); err != nil {
		if _, ok := err.(user.UnknownUserError); !ok {
			return err
		}
		if err = createUser(id); err != nil {
			return err
		}
	}

	if d, err = docker.GetConnection(dockerSocket); err != nil {
		return err
	}

	if imgInfo, err = d.GetImage(imageName); err != nil {
		return err
	}

	if err := os.MkdirAll(id.HomePath(), 0700); err != nil {
		return err
	}

	u, _ := user.Lookup(id.LoginFor())
	volumes := make([]string, 0, 10)
	for volPath := range imgInfo.Config.Volumes {
		volumes = append(volumes, volPath)
	}

	user := imgInfo.Config.User
	if user == "" {
		user = "container"
	}

	ports, err := GetExistingPorts(id)
	if err != nil {
		fmt.Printf("container init pre-start: Unable to retrieve port mapping\n")
		return err
	}

	containerData := ContainerInitScript{
		imgInfo.Config.User == "",
		user,
		u.Uid,
		u.Gid,
		strings.Join(imgInfo.Config.Cmd, " "),
		len(volumes) > 0,
		strings.Join(volumes, " "),
		ports,
		socketActivationType == "proxied",
	}

	file, _, err := utils.OpenFileExclusive(path.Join(id.HomePath(), "container-init.sh"), 0700)
	if err != nil {
		fmt.Printf("container init pre-start: Unable to open script file: %v\n", err)
		return err
	}
	defer file.Close()

	if erre := ContainerInitTemplate.Execute(file, containerData); erre != nil {
		fmt.Printf("container init pre-start: Unable to output template: ", erre)
		return erre
	}
	if err := file.Close(); err != nil {
		return err
	}

	file, _, err = utils.OpenFileExclusive(path.Join(id.HomePath(), "container-cmd.sh"), 0705)
	if err != nil {
		fmt.Printf("container init pre-start: Unable to open cmd script file: %v\n", err)
		return err
	}
	defer file.Close()

	if erre := ContainerCmdTemplate.Execute(file, containerData); erre != nil {
		fmt.Printf("container init pre-start: Unable to output cmd template: ", erre)
		return erre
	}
	if err := file.Close(); err != nil {
		return err
	}

	return nil
}

func createUser(id Identifier) error {
	cmd := exec.Command("/usr/sbin/useradd", id.LoginFor(), "-m", "-d", id.HomePath(), "-c", "Container user")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Println(out)
		return err
	}
	selinux.RestoreCon(id.HomePath(), true)
	return nil
}

func InitPostStart(dockerSocket string, id Identifier) error {
	var (
		u         *user.User
		container *dc.Container
		err       error
		d         *docker.DockerClient
	)

	if u, err = user.Lookup(id.LoginFor()); err != nil {
		return err
	}

	if d, err = docker.GetConnection(dockerSocket); err != nil {
		return err
	}

	if err := GenerateAuthorizedKeys(id, u, true, false); err != nil {
		return err
	}

	const CONTAINER_RUNNING_WAIT_TIME = 10
	for i := 0; i < CONTAINER_RUNNING_WAIT_TIME; i++ {
		if container, err = d.GetContainer(id.ContainerFor(), true); err != nil {
			return err
		}
		if container.State.Running {
			break
		} else {
			log.Printf("Waiting for container to run.")
			time.Sleep(time.Second)
		}
	}

	if file, err := os.Open(id.NetworkLinksPathFor()); err == nil {
		defer file.Close()
		pid, err := d.ChildProcessForContainer(container)
		if err != nil {
			return err
		}
		if pid < 2 {
			return errors.New("support: child PID is not correct")
		}
		log.Printf("Updating network namespaces for %d", pid)
		if err := updateNamespaceNetworkLinks(pid, "127.0.0.2", file); err != nil {
			return err
		}
	}

	return nil
}

func GenerateAuthorizedKeys(id Identifier, u *user.User, forceCreate bool, printToStdOut bool) error {
	var (
		err      error
		sshKeys  []string
		destFile *os.File
		srcFile  *os.File
		w        *bufio.Writer
	)

	var authorizedKeysPortSpec string
	ports, err := GetExistingPorts(id)
	if err != nil {
		fmt.Errorf("container init pre-start: Unable to retrieve port mapping")
		return err
	}

	for _, port := range ports {
		authorizedKeysPortSpec += fmt.Sprintf("permitopen=\"127.0.0.1:%v\",", port.External)
	}

	sshKeys, err = filepath.Glob(path.Join(id.SshAccessBasePath(), "*"))

	if !printToStdOut {
		os.MkdirAll(id.HomePath(), 0700)
		os.Mkdir(path.Join(id.HomePath(), ".ssh"), 0700)
		authKeysPath := id.AuthKeysPathFor()
		if _, err = os.Stat(authKeysPath); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			if forceCreate {
				os.Remove(authKeysPath)
			} else {
				return nil
			}
		}

		if destFile, err = os.Create(authKeysPath); err != nil {
			return err
		}
		defer destFile.Close()
		w = bufio.NewWriter(destFile)
	} else {
		w = bufio.NewWriter(os.Stdout)
	}

	for _, keyFile := range sshKeys {
		s, err := os.Stat(keyFile)
		if err != nil {
			continue
		}
		if s.IsDir() {
			continue
		}

		srcFile, err = os.Open(keyFile)
		defer srcFile.Close()
		w.WriteString(fmt.Sprintf("command=\"%v/bin/switchns\",%vno-agent-forwarding,no-X11-forwarding ", config.ContainerBasePath(), authorizedKeysPortSpec))
		io.Copy(w, srcFile)
		w.WriteString("\n")
	}
	w.Flush()

	if !printToStdOut {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)

		for _, path := range []string{
			id.HomePath(),
			filepath.Join(id.HomePath(), ".ssh"),
			filepath.Join(id.HomePath(), ".ssh", "authorized_keys"),
		} {
			if err := os.Chown(path, uid, gid); err != nil {
				return err
			}
		}

		if err := selinux.RestoreCon(id.BaseHomePath(), true); err != nil {
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
		log.Printf("network_links: Could not read IP for container: %v", erro)
		return nil, erro
	}
	sourceAddr, errr := net.ResolveIPAddr("ip", strings.TrimSpace(string(source)))
	if errr != nil {
		log.Printf("network_links: Host source IP %s does not resolve %v", sourceAddr, errr)
		return nil, errr
	}
	return sourceAddr, nil
}

type addressResolver struct {
	local   net.IP
	checked bool
}

func (resolver *addressResolver) ResolveIP(host string) (net.IP, error) {
	if host == "localhost" || host == "127.0.0.1" {
		if resolver.local != nil {
			return resolver.local, nil
		}
		if !resolver.checked {
			resolver.checked = true
			devices, err := net.Interfaces()
			if err != nil {
				return nil, err
			}
			for _, dev := range devices {
				if (dev.Flags&net.FlagUp != 0) && (dev.Flags&net.FlagLoopback == 0) {
					addrs, err := dev.Addrs()
					if err != nil {
						continue
					}
					for i := range addrs {
						if ip, ok := addrs[i].(*net.IPNet); ok {
							log.Printf("Using %v for %s", ip, host)
							resolver.local = ip.IP
							return resolver.local, nil
						}
					}
				}
			}
		}
	}
	addr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, err
	}
	return addr.IP, nil
}

func updateNamespaceNetworkLinks(pid int, localAddr string, ports io.Reader) error {
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
		log.Printf("network_links: Failed to enable localnet routing: %v", err)
		log.Printf("network_links: error output: %v", output)
		return err
	}

	// Enable ip forwarding
	output, err = exec.Command("ip", "netns", "exec", name, "sysctl", "-w", "net.ipv4.ip_forward=1").Output()
	if err != nil {
		log.Printf("network_links: Failed to enable ipv4 forwarding: %v", err)
		log.Printf("network_links: error output: %v", output)
		return err
	}

	// Restore a set of rules to the table
	cmd := exec.Command("ip", "netns", "exec", name, "iptables-restore")
	stdin, errp := cmd.StdinPipe()
	if errp != nil {
		log.Printf("network_links: Could not open pipe to iptables-restore: %v", errp)
		return errp
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		log.Printf("network_links: Could not start iptables-restore: %v", errp)
		return err
	}

	fmt.Fprintf(stdin, "*nat\n")
	for {
		link := NetworkLink{}
		if _, err := fmt.Fscanf(ports, "%v\t%v\t%s\n", &link.FromPort, &link.ToPort, &link.ToHost); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("network_links: Could not read from network links file: %v", err)
			continue
		}
		if err := link.Check(); err != nil {
			log.Printf("network_links: Link in file is not valid: %v", err)
			continue
		}
		if link.Complete() {
			destIP, err := resolver.ResolveIP(link.ToHost)
			if err != nil {
				log.Printf("network_links: Link destination host does not resolve %v", err)
				continue
			}

			data := OutboundNetworkIptables{sourceAddr.String(), localAddr, link.FromPort, destIP.String(), link.ToPort}
			if err := OutboundNetworkIptablesTemplate.Execute(stdin, &data); err != nil {
				log.Printf("network_links: Unable to write network link rules: %v", err)
				return err
			}
		}
	}
	fmt.Fprintf(stdin, "COMMIT\n")

	stdin.Close()
	if err := cmd.Wait(); err != nil {
		log.Printf("network_links: iptables-restore did not successfully complete: %v", err)
		return err
	}
	return nil
}

func GetContainerIPs(d *docker.DockerClient, ids []Identifier) (map[string]Identifier, error) {
	ips := make(map[string]Identifier)
	for _, id := range ids {
		if cInfo, err := d.GetContainer(id.ContainerFor(), false); err == nil {
			ips[cInfo.NetworkSettings.IPAddress] = id
		}
	}
	return ips, nil
}
