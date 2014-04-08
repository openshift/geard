// +build !disable_ssh

package cmd

import (
	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/http"
	sshhttp "github.com/openshift/geard/ssh/http"
)

func init() {
	http.AddHttpExtension(sshhttp.Routes)

	cmd.AddCommandExtension(registerLocal, true)
	cmd.AddCommandExtension(registerRemote, false)
}
