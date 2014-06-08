package init

import (
	"errors"
	"fmt"
	dc "github.com/fsouza/go-dockerclient"
	"github.com/spf13/cobra"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/containers/systemd"
	"github.com/openshift/geard/docker"
	"github.com/openshift/geard/selinux"
	"github.com/openshift/geard/ssh"
	"github.com/openshift/geard/utils"
)

var (
	pre          bool
	post         bool
	dockerSocket string
)

func RegisterInit(parent *cobra.Command) {
	initGearCmd := &cobra.Command{
		Use:   "init <name> <image>",
		Short: "(Local) Setup the environment for a container",
		Long:  "",
		Run:   initGear,
	}
	initGearCmd.Flags().BoolVarP(&pre, "pre", "", false, "Perform pre-start initialization")
	initGearCmd.Flags().BoolVarP(&post, "post", "", false, "Perform post-start initialization")
	initGearCmd.Flags().StringVarP(&dockerSocket, "docker-socket", "S", "unix:///var/run/docker.sock", "Set the docker socket to use")
	parent.AddCommand(initGearCmd)
}

func initGear(c *cobra.Command, args []string) {
	if len(args) != 2 || !(pre || post) || (pre && post) {
		cmd.Fail(1, "Valid arguments: <id> <image_name> (--pre|--post)")
	}
	containerId, err := containers.NewIdentifier(args[0])
	if err != nil {
		cmd.Fail(1, "Argument 1 must be a valid gear identifier: %s", err.Error())
	}

	dockerSocket := c.Flags().Lookup("docker-socket").Value.String()

	switch {
	case pre:
		if err := initPreStart(dockerSocket, containerId, args[1]); err != nil {
			cmd.Fail(2, "Unable to initialize container %s", err.Error())
		}
	case post:
		if err := initPostStart(dockerSocket, containerId); err != nil {
			cmd.Fail(2, "Unable to initialize container %s", err.Error())
		}
	}
}

var resolver addressResolver = addressResolver{}

func initPreStart(dockerSocket string, id containers.Identifier, imageName string) error {
	var (
		err     error
		imgInfo *dc.Image
		d       *docker.DockerClient
	)

	_, socketActivationType, err := systemd.GetSocketActivation(id)
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

	ports, err := containers.GetExistingPorts(id)
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

	file, _, err := utils.OpenFileExclusive(path.Join(id.RunPathFor(), "container-init.sh"), 0700)
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

	file, _, err = utils.OpenFileExclusive(path.Join(id.RunPathFor(), "container-cmd.sh"), 0705)
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

func createUser(id containers.Identifier) error {
	cmd := exec.Command("/usr/sbin/useradd", id.LoginFor(), "-m", "-d", id.HomePath(), "-c", "Container user")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Println(out)
		return err
	}
	selinux.RestoreCon(id.HomePath(), true)
	return nil
}

func initPostStart(dockerSocket string, id containers.Identifier) error {
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

		const ContainerInterval = time.Second / 10
		const ContainerWait = time.Second * 15
		for i := 0; i < int(ContainerWait/ContainerInterval); i++ {
			if container, err = d.InspectContainer(id.ContainerFor()); err != nil {
				if err == docker.ErrNoSuchContainer {
					//log.Printf("Waiting for container to be available.")
					time.Sleep(ContainerInterval)
					continue
				}
				return err
			}
			if container.State.Running && container.State.Pid != 0 {
				break
			} else {
				//log.Printf("Waiting for container to report available.")
				time.Sleep(ContainerInterval)
			}
		}

		if container == nil {
			return fmt.Errorf("container %s was not visible through Docker before timeout", id.ContainerFor())
		}

		pid, err := d.ChildProcessForContainer(container)
		if err != nil {
			return err
		}
		if pid <= 1 {
			return errors.New("child PID is not correct")
		}

		name, errl := linkNetworkNamespace(pid)
		if errl != nil {
			return errl
		}
		defer unlinkNetworkNamespace(pid)

		var sourceAddr *net.IPAddr
		errs := errors.New("IP never became available")
		for i := 0; i < int(ContainerWait/ContainerInterval); i++ {
			if sourceAddr, errs = getHostIPFromNamespace(name); errs == nil {
				break
			}
			time.Sleep(ContainerInterval)
		}
		if sourceAddr == nil {
			return fmt.Errorf("unable to get the container's IP address: %s", errs.Error())
		}

		log.Printf("Updating network namespaces for %d", pid)
		if err := updateNamespaceNetworkLinks(name, sourceAddr, file); err != nil {
			return err
		}
	}

	return nil
}

func linkNetworkNamespace(pid int) (string, error) {
	name := "netlink-" + strconv.Itoa(pid)
	path := fmt.Sprintf("/var/run/netns/%s", name)
	nsPath := fmt.Sprintf("/proc/%d/ns/net", pid)
	if err := os.MkdirAll("/var/run/netns", 0755); err != nil {
		return name, err
	}
	if err := os.Symlink(nsPath, path); err != nil && !os.IsExist(err) {
		return name, err
	}
	return name, nil
}

func unlinkNetworkNamespace(pid int) error {
	name := "netlink-" + strconv.Itoa(pid)
	path := fmt.Sprintf("/var/run/netns/%s", name)
	return os.Remove(path)
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
							if ip.IP.To4() != nil {
								log.Printf("Using %v for %s", ip, host)
								resolver.local = ip.IP
								return resolver.local, nil
							}
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

func updateNamespaceNetworkLinks(name string, sourceAddr *net.IPAddr, ports io.Reader) error {

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

			destIP, err := resolver.ResolveIP(link.ToHost)
			if err != nil {
				log.Printf("gear: Link destination host does not resolve %v", err)
				continue
			}

			log.Printf("Mapping %s(%s):%d -> %s:%d", sourceAddr.String(), srcIP.String(), link.FromPort, destIP.String(), link.ToPort)

			data := OutboundNetworkIptables{sourceAddr.String(), srcIP.IP.String(), link.FromPort, destIP.String(), link.ToPort}
			if err := OutboundNetworkIptablesTemplate.Execute(stdin, &data); err != nil {
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
