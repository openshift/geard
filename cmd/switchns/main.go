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
	containerName string
	git_rw        bool
	git_ro        bool
	envs          Environment
	reaminingArgs []string
)

func main() {
	switchnsCmd := &cobra.Command{
		Use:   "switchns",
		Short: "Run commands within containers or repositories",
		Run:   switchns,
	}
	switchnsCmd.Flags().VarP(&(envs), "env", "", "Specify environment variable to set in KEY=VALUE format")
	switchnsCmd.Flags().StringVarP(&(containerName), "container", "", "", "Container name or ID")
	switchnsCmd.Flags().BoolVar(&(git_rw), "git", false, "Enter a git container in read-write mode")
	switchnsCmd.Flags().BoolVar(&(git_ro), "git-ro", false, "Enter a git container in read-write mode")

	args := []string{}
	for idx, arg := range os.Args[1:] {
		if arg != "--" {
			args = append(args, arg)
		} else {
			reaminingArgs = os.Args[idx+2:]
			break
		}
	}

	switchnsCmd.SetArgs(args)
	if err := switchnsCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func switchns(cmd *cobra.Command, args []string) {
	if git_ro || git_rw {
		switchnsGit(cmd, reaminingArgs)
	} else {
		switchnsExec(cmd, reaminingArgs)
	}
}

func switchnsExec(cmd *cobra.Command, args []string) {
	var err error

	uid := os.Getuid()

	if uid == 0 {
		runCommand(containerName, args, envs)
	} else {
		var u *user.User
		var containerId containers.Identifier

		if u, err = user.LookupId(strconv.Itoa(uid)); err != nil {
			os.Exit(2)
		}

		if containerId, err = containers.NewIdentifierFromUser(u); err != nil {
			os.Exit(2)
		}
		runCommand(containerId.ContainerFor(), []string{"/bin/bash", "-l"}, []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"})
	}
}

func switchnsGit(cmd *cobra.Command, args []string) {
	var u *user.User
	var err error
	var repoId git.RepoIdentifier

	uid := os.Getuid()
	originalCommand := os.Getenv("SSH_ORIGINAL_COMMAND")

	if u, err = user.LookupId(strconv.Itoa(uid)); err != nil {
		os.Exit(2)
	}

	if uid != 0 {
		if !isValidGitCommand(originalCommand, !git_rw) {
			os.Exit(2)
		}
		if repoId, err = git.NewIdentifierFromUser(u); err != nil {
			os.Exit(2)
		}
		env := []string{fmt.Sprintf("HOME=%s", repoId.RepositoryPathFor())}
		runCommand("geard-githost", []string{"/usr/bin/git-shell", "-c", originalCommand}, env)
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

func runCommand(name string, command []string, environment []string) {
	client, err := docker.GetConnection("unix:///var/run/docker.sock")
	if err != nil {
		fmt.Printf("Unable to connect to server\n")
		os.Exit(3)
	}

	container, err := client.GetContainer(name, false)
	if err != nil {
		fmt.Printf("Unable to locate container named %v\n", name)
		os.Exit(3)
	}
	containerNsPID, err := client.ChildProcessForContainer(container)
	if err != nil {
		os.Exit(3)
	}
	namespace.RunIn(name, containerNsPID, command, environment)
}
