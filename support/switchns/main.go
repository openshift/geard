package main

import (
	"fmt"
	"github.com/docopt/docopt.go"
	"github.com/smarterclayton/geard/docker"
	"github.com/smarterclayton/geard/support/switchns/namespace"
	"os"
	"os/user"
	"strconv"
)

const usage = `Switch into container namespace and execute a command.

If run by root user, allows you to specify the container name to enter and command to run.
If executed by a non-root user, enters the container matching the login name.

Usage:
	switchns <container name> [--env="key=value"]... [--] <command>...
	
Examples:
	switchns gear-0001 /bin/echo 1
	switchns gear-0001 -- /bin/bash -c "echo \$PATH"
	switchns gear-0001 --env FOO=BAR --env BAZ=ZAB -- /bin/bash -c "echo \$FOO \$BAZ"
`

func main() {
	var arguments map[string]interface{}
	var err error
	uid := os.Getuid()

	if uid == 0 {
		if arguments, err = docopt.Parse(usage, nil, true, "switchns", false); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		containerName := (arguments["<container name>"]).(string)
		command := (arguments["<command>"]).([]string)
		env := []string{}
		if arguments["--env"] != nil {
			env = (arguments["--env"]).([]string)
		}

		_, container, errc := docker.GetContainer("unix:///var/run/docker.sock", containerName, false)
		if errc != nil {
			fmt.Printf("Unable to locate container named %v", containerName)
			os.Exit(3)
		}
		containerNsPID, err := docker.ChildProcessForContainer(container)
		if err != nil {
			fmt.Printf("Unable to locate process for container named %v", containerName)
			os.Exit(3)
		}
		namespace.RunIn(string(containerName), containerNsPID, command, env)
	} else {
		var u *user.User
		if u, err = user.LookupId(strconv.Itoa(uid)); err != nil {
			os.Exit(2)
		}
		_, container, err := docker.GetContainer("unix:///var/run/docker.sock", u.Username, false)
		if err != nil {
			fmt.Printf("Unable to locate container named %v", u.Username)
			os.Exit(3)
		}
		containerNsPID, err := docker.ChildProcessForContainer(container)
		if err != nil {
			os.Exit(3)
		}
		namespace.RunIn(u.Username, containerNsPID, []string{"/bin/bash", "-l"}, []string{})
	}
}
