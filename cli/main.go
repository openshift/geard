package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/crosbymichael/libcontainer"
	"github.com/crosbymichael/libcontainer/namespaces"
	"github.com/crosbymichael/libcontainer/network"
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

func exec(contianer *libcontainer.Container, name string) error {
	driver := namespaces.New()

	f, err := os.Open("/root/nsroot/test")
	if err != nil {
		return err
	}
	contianer.NetworkNamespace = f.Fd()

	pid, err := driver.Exec(contianer)
	if err != nil {
		return fmt.Errorf("error exec container %s", err)
	}
	if displayPid {
		fmt.Println(pid)
	}
	body, err := json.Marshal(contianer)
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

	exitcode, err := libcontainer.WaitOnPid(pid)
	if err != nil {
		return fmt.Errorf("error waiting on child %s", err)
	}
	os.Exit(exitcode)
	return nil
}

func execIn(container *libcontainer.Container) error {
	driver := namespaces.New()
	pid, err := driver.ExecIn(container, &libcontainer.Command{
		Env: container.Command.Env,
		Args: []string{
			newCommand,
		},
	})
	if err != nil {
		return fmt.Errorf("error exexin container %s", err)
	}
	if pid != 0 { // fix exec in returning pid of 0, we do two forks :(
		exitcode, err := libcontainer.WaitOnPid(pid)
		if err != nil {
			return fmt.Errorf("error waiting on child %s", err)
		}
		os.Exit(exitcode)
	}
	return nil
}

func createNet(config *libcontainer.Network) error {
	root := "/root/nsroot"
	if err := namespaces.SetupNamespaceMountDir(root); err != nil {
		return err
	}

	nspath := root + "/test"
	pid, err := namespaces.CreateNetworkNamespace(nspath)
	if err != nil {
		return nil
	}
	exit, err := libcontainer.WaitOnPid(pid)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("exit code not 0")
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

	if pid, err = namespaces.SetupNetworkNamespace(f.Fd(), config); err != nil {
		return err
	}
	exit, err = libcontainer.WaitOnPid(pid)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("exit code not 0")
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
