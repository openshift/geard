package main

import (
	"fmt"
	"github.com/docopt/docopt.go"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/docker"
	"github.com/smarterclayton/geard/git"
	"github.com/smarterclayton/geard/support/switchns/namespace"
	"os"
	"os/user"
	"strconv"
	"strings"
)

const usage = `Switch into container namespace and execute a command.

If run by root user, allows you to specify the container name to enter and command to run.
If executed by a non-root user, enters the container matching the login name.

If executed by a non-root user with --git or --git-ro option, enters the git container
and runs the git command from the SSH_ORIGINAL_COMMAND environment variable

Usage:
    switchns [--git|--git-ro]
    switchns <container name> [--env="key=value"]... [--] <command>...
	
Examples:
    switchns container-0001 /bin/echo 1
    switchns container-0001 -- /bin/bash -c "echo \$PATH"
    switchns container-0001 --env FOO=BAR --env BAZ=ZAB -- /bin/bash -c "echo \$FOO \$BAZ"
`

func main() {
	var arguments map[string]interface{}
	var err error
	uid := os.Getuid()
	if arguments, err = docopt.Parse(usage, nil, true, "switchns", false); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if uid == 0 {
		containerName := (arguments["<container name>"]).(string)
		command := (arguments["<command>"]).([]string)
		env := []string{}
		if arguments["--env"] != nil {
			env = (arguments["--env"]).([]string)
		}

		runCommand(containerName, command, env)
	} else {
		var u *user.User
		var repoId git.RepoIdentifier
		var containerId containers.Identifier
		originalCommand := os.Getenv("SSH_ORIGINAL_COMMAND")

		if u, err = user.LookupId(strconv.Itoa(uid)); err != nil {
			os.Exit(2)
		}

		if arguments["--git"].(bool) || arguments["--git-ro"].(bool) {
			if !isValidGitCommand(originalCommand, arguments["--git-ro"].(bool)) {
				os.Exit(2)
			}
			if repoId, err = git.NewIdentifierFromUser(u); err != nil {
				os.Exit(2)
			}
			env := []string{fmt.Sprintf("HOME=%s", repoId.RepositoryPathFor())}
			runCommand("geard-git-host", []string{"/usr/bin/git-shell", "-c", originalCommand}, env)
		} else {
			if containerId, err = containers.NewIdentifierFromUser(u); err != nil {
				os.Exit(2)
			}
			runCommand(containerId.ContainerFor(), []string{"/bin/bash", "-l"}, []string{})
		}
	}
}

func isValidGitCommand(command string, isReadOnlyUser bool) bool {
	if !(strings.HasPrefix(command, "git-receive-pack") || strings.HasPrefix(command, "git-upload-pack") || strings.HasPrefix(command, "git-upload-archive")) {
		return false
	}
	if isReadOnlyUser && strings.HasPrefix(command, "git-receive-pack") {
		return false
	}
	return true
}

func runCommand(name string, command []string, environment []string) {
	client, err := docker.GetConnection("unix:///var/run/docker.sock")
	if err != nil {
		fmt.Printf("Unable to connect to server")
		os.Exit(3)
	}

	container, err := client.GetContainer(name, false)
	if err != nil {
		fmt.Printf("Unable to locate container named %v", name)
		os.Exit(3)
	}
	containerNsPID, err := client.ChildProcessForContainer(container)
	if err != nil {
		os.Exit(3)
	}
	namespace.RunIn(name, containerNsPID, command, environment)
}
