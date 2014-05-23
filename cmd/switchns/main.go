// An executable for executing a process inside a running Docker container.  Can be used as
// root to switch into any named container (name is the same as the gear name), or as a
// container user (user tied to a container) to enter the context for SSH or other function.
// Will be eventually become a setuid stub for docker exec.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift/geard/cmd/switchns/namespace"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/docker"
	"github.com/openshift/geard/git"
)

type Environment []string

func (e *Environment) Set(value string) error {
	*e = append(*e, value)
	return nil
}

func (e *Environment) String() string {
	return fmt.Sprint([]string(*e))
}

var (
	containerName   string
	gitRw           bool
	gitRo           bool
	envs            Environment
	passthroughArgs []string
)

func main() {
	switchnsCmd := &cobra.Command{
		Use:   "switchns",
		Short: "Run commands within containers or repositories",
		Run:   switchns,
	}
	switchnsCmd.Flags().VarP(&envs, "env", "", "Specify environment variable to set in KEY=VALUE format")
	switchnsCmd.Flags().StringVarP(&containerName, "container", "", "", "Container name or ID")
	switchnsCmd.Flags().BoolVar(&gitRw, "git", false, "Enter a git container in read-write mode")
	switchnsCmd.Flags().BoolVar(&gitRo, "git-ro", false, "Enter a git container in read-write mode")

	var commandArgs []string
	commandArgs, passthroughArgs = extractPassthroughArgs()

	switchnsCmd.SetArgs(commandArgs)

	if err := switchnsCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

// Parses os.Args and returns the left and right side of `--` as arrays.
func extractPassthroughArgs() (left []string, right []string) {
	if len(os.Args) > 0 {
		for i, arg := range os.Args[1:] {
			if arg != "--" {
				left = append(left, arg)
			} else {
				right = os.Args[i+2:]
				break
			}
		}
	}

	return left, right
}

func switchns(cmd *cobra.Command, args []string) {
	if gitRo || gitRw {
		switchnsGit()
	} else {
		switchnsExec(passthroughArgs)
	}
}

func switchnsExec(args []string) {
	var err error

	uid := os.Getuid()

	if uid == 0 {
		runCommandInContainer(containerName, args, envs)
	} else {
		var u *user.User
		var containerId containers.Identifier

		if u, err = user.LookupId(strconv.Itoa(uid)); err != nil {
			fmt.Printf("Couldn't lookup uid %s\n", uid)
			os.Exit(2)
		}

		if containerId, err = containers.NewIdentifierFromUser(u); err != nil {
			fmt.Printf("Couldn't get identifier from user: %v\n", u)
			os.Exit(2)
		}
		runCommandInContainer(containerId.ContainerFor(), []string{"/bin/bash", "-l"}, []string{})
	}
}

func switchnsGit() {
	var u *user.User
	var err error
	var repoId git.RepoIdentifier

	uid := os.Getuid()
	originalCommand := os.Getenv("SSH_ORIGINAL_COMMAND")

	if u, err = user.LookupId(strconv.Itoa(uid)); err != nil {
		fmt.Printf("Couldn't find user with uid %n\n", uid)
		os.Exit(2)
	}

	if uid != 0 {
		if !isValidGitCommand(originalCommand, !gitRw) {
			fmt.Printf("Invalid git command: %s\n", originalCommand)
			os.Exit(2)
		}
		if repoId, err = git.NewIdentifierFromUser(u); err != nil {
			fmt.Printf("Couldn't create identifier for user %v\n", u)
			os.Exit(2)
		}
		env := []string{fmt.Sprintf("HOME=%s", repoId.RepositoryPathFor())}
		runCommandInContainer("geard-githost", []string{"/usr/bin/git-shell", "-c", originalCommand}, env)
	} else {
		fmt.Println("Cannot switch into any git repo as root user")
		os.Exit(2)
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

func runCommandInContainer(name string, command []string, environment []string) {
	if len(command) == 0 {
		fmt.Println("No command specified")
		os.Exit(3)
	}

	client, err := docker.GetConnection("unix:///var/run/docker.sock")
	if err != nil {
		fmt.Printf("Unable to connect to server\n")
		os.Exit(3)
	}

	container, err := client.InspectContainer(name)
	if err != nil {
		fmt.Printf("Unable to locate container named %v\n", name)
		os.Exit(3)
	}
	containerNsPID, err := client.ChildProcessForContainer(container)
	if err != nil {
		fmt.Println("Couldn't create child process for container")
		os.Exit(3)
	}

	containerEnv := environment

	if len(containerEnv) == 0 {
		containerEnv = container.Config.Env
	}

	namespace.RunIn(name, containerNsPID, command, containerEnv)
}
