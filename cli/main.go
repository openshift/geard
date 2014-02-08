package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/crosbymichael/libcontainer"
	"github.com/crosbymichael/libcontainer/namespaces"
	"os"
)

var (
	displayPid bool
)

func init() {
	flag.BoolVar(&displayPid, "pid", false, "display the pid before waiting")
	flag.Parse()
}

func exec(contianer *libcontainer.Container, name string) error {
	driver := namespaces.New()

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

	f, err := os.OpenFile(name, os.O_RDWR, 0755)
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
			"/bin/bash",
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
	default:
		err = fmt.Errorf("command not supported: %s", cliCmd)
	}

	if err != nil {
		printErr(err)
	}
}
