package cmd

import (
	"os"
	"os/user"

	"github.com/openshift/cobra"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/git"
)

func ExecuteSshAuthKeysCmd(args ...string) {
	if len(args) != 2 {
		os.Exit(2)
	}
	SshAuthKeysCommand(nil, args[1:])
}

func SshAuthKeysCommand(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		Fail(1, "Valid arguments: <login name>\n")
	}

	var (
		u           *user.User
		err         error
		containerId containers.Identifier
		repoId      git.RepoIdentifier
	)

	if u, err = user.Lookup(args[0]); err != nil {
		Fail(2, "Unable to lookup user")
	}

	isRepo := u.Name == "Repository user"
	if isRepo {
		repoId, err = git.NewIdentifierFromUser(u)
		if err != nil {
			Fail(1, "Not a repo user: %s\n", err.Error())
		}
	} else {
		containerId, err = containers.NewIdentifierFromUser(u)
		if err != nil {
			Fail(1, "Not a container user: %s\n", err.Error())
		}
	}

	if isRepo {
		if err := git.GenerateAuthorizedKeys(repoId, u, false, true); err != nil {
			Fail(2, "Unable to generate authorized_keys file: %s\n", err.Error())
		}
	} else {
		if err := containers.GenerateAuthorizedKeys(containerId, u, false, true); err != nil {
			Fail(2, "Unable to generate authorized_keys file: %s\n", err.Error())
		}
	}
}
