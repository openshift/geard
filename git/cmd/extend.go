// +build !disable_git

package cmd

import (
	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/git"
	githttp "github.com/openshift/geard/git/http"
	"github.com/openshift/geard/http"
	sshcmd "github.com/openshift/geard/ssh/cmd"
)

func init() {
	cmd.AddInitializer(git.InitializeData, cmd.ForDaemon)

	http.AddHttpExtension(&githttp.HttpExtension{})

	cmd.AddCommandExtension(registerRemote, false)
	cmd.AddCommandExtension(registerLocal, true)

	sshcmd.AddPermissionCommand(git.ResourceTypeRepository, &handler)
}
