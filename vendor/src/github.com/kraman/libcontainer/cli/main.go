package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/kraman/libcontainer"
	"github.com/kraman/libcontainer/namespaces"
	"github.com/kraman/libcontainer/network"
	"github.com/kraman/libcontainer/utils"
	"os"
)

var (
	displayPid bool
	newCommand string
)

func init() {
	flag.BoolVar(&displayPid, "pid", false, "display the pid before waiting")
	flag.StringVar(&newCommand, "cmd", "/bin/bash", "command to run in the existing namespace")
	flag.Parse()
}

func exec(container *libcontainer.Container, name string) error {
	f, err := os.Open("/root/nsroot/test")
	if err != nil {
		return err
	}
	container.NetNsFd = f.Fd()

	pid, err := namespaces.Exec(container)
	if err != nil {
		return fmt.Errorf("error exec container %s", err)
	}

	container.NsPid = pid
	if displayPid {
		fmt.Println(pid)
	}

	body, err := json.Marshal(container)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)
	if err := json.Indent(buf, body, "", "    "); err != nil {
		return err
	}

	f, err = os.OpenFile(name, os.O_RDWR, 0755)
	if err != nil {
		return err
	}
	if _, err := buf.WriteTo(f); err != nil {
		f.Close()
		return err
	}
	f.Close()

	exitcode, err := utils.WaitOnPid(pid)
	if err != nil {
		return fmt.Errorf("error waiting on child %s", err)
	}
	if err := network.DeleteNetworkNamespace("/root/nsroot/test"); err != nil {
		return err
	}
	os.Exit(exitcode)
	return nil
}

func execIn(container *libcontainer.Container) error {
	f, err := os.Open("/root/nsroot/test")
	if err != nil {
		return err
	}
	container.NetNsFd = f.Fd()
	pid, err := namespaces.ExecIn(container, &libcontainer.Command{
		Env: container.Command.Env,
		Args: []string{
			newCommand,
		},
	})
	if err != nil {
		return fmt.Errorf("error exexin container %s", err)
	}
	exitcode, err := utils.WaitOnPid(pid)
	if err != nil {
		return fmt.Errorf("error waiting on child %s", err)
	}
	os.Exit(exitcode)
	return nil
}

func createNet(config *libcontainer.Network) error {
	root := "/root/nsroot"
	if err := network.SetupNamespaceMountDir(root); err != nil {
		return err
	}

	nspath := root + "/test"
	if err := network.CreateNetworkNamespace(nspath); err != nil {
		return nil
	}
	if err := network.CreateVethPair("veth0", config.TempVethName); err != nil {
		return err
	}
	if err := network.SetInterfaceMaster("veth0", config.Bridge); err != nil {
		return err
	}
	if err := network.InterfaceUp("veth0"); err != nil {
		return err
	}

	f, err := os.Open(nspath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := network.SetInterfaceInNamespaceFd("veth1", int(f.Fd())); err != nil {
		return err
	}

	if err := network.SetupVethInsideNamespace(f.Fd(), config); err != nil {
		return err
	}
	return nil
}

func printErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	var (
		err    error
		cliCmd = flag.Arg(0)
		config = flag.Arg(1)
	)
	f, err := os.Open(config)
	if err != nil {
		printErr(err)
	}

	dec := json.NewDecoder(f)
	var container *libcontainer.Container

	if err := dec.Decode(&container); err != nil {
		printErr(err)
	}
	f.Close()

	switch cliCmd {
	case "exec":
		err = exec(container, config)
	case "execin":
		err = execIn(container)
	case "net":
		err = createNet(&libcontainer.Network{
			TempVethName: "veth1",
			IP:           "172.17.0.100/16",
			Gateway:      "172.17.42.1",
			Mtu:          1500,
			Bridge:       "docker0",
		})
	default:
		err = fmt.Errorf("command not supported: %s", cliCmd)
	}

	if err != nil {
		printErr(err)
	}
}
