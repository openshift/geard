// +build linux && !skip_default

package main

import (
	cleancmd "github.com/openshift/geard/cleanup/cmd"
	"github.com/openshift/geard/cmd"
	ctrcmd "github.com/openshift/geard/containers/cmd"
	chttp "github.com/openshift/geard/containers/http"
	cjobs "github.com/openshift/geard/containers/jobs/linux"
	initcmd "github.com/openshift/geard/containers/systemd/init"
	"github.com/openshift/geard/daemon"
	daemoncmd "github.com/openshift/geard/daemon/cmd"
	"github.com/openshift/geard/git"
	gitcmd "github.com/openshift/geard/git/cmd"
	githttp "github.com/openshift/geard/git/http"
	gitjobs "github.com/openshift/geard/git/jobs/linux"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/http/client"
	httpcmd "github.com/openshift/geard/http/cmd"
	"github.com/openshift/geard/jobs"
	routercmd "github.com/openshift/geard/router/cmd"
	routerhttp "github.com/openshift/geard/router/http"
	routerjobs "github.com/openshift/geard/router/jobs/linux"
	sshcmd "github.com/openshift/geard/ssh/cmd"
	sshhttp "github.com/openshift/geard/ssh/http"
	sshjobs "github.com/openshift/geard/ssh/jobs"
	"github.com/openshift/geard/transport"
)

func init() {
	transport.RegisterTransport("http", &client.HttpTransport{})
	defaultTransport.Set("http")

	daemon.AddDaemonExtension(&httpcmd.Daemon{})
	cmd.AddCommandExtension((&daemoncmd.Command{"http://127.0.0.1:43273"}).RegisterLocal, true)

	ctx := ctrcmd.CommandContext{Transport: &defaultTransport.TransportFlag, Insecure: &insecure}
	cmd.AddCommandExtension(ctx.RegisterLocal, true)
	cmd.AddCommandExtension(ctx.RegisterRemote, false)

	c := &routercmd.Command{&defaultTransport.TransportFlag}
	cmd.AddCommandExtension(c.RegisterRouterCmds, true)

	cmd.AddCommandExtension(gitcmd.RegisterInitRepo, true)
	cmd.AddCommandExtension((&gitcmd.CommandContext{&defaultTransport.TransportFlag}).RegisterCreateRepo, false)

	sshcmd.AddPermissionCommand(git.ResourceTypeRepository, &gitcmd.PermissionCommandContext{})

	cmd.AddCommandExtension(sshcmd.RegisterAuthorizedKeys, true)
	cmd.AddCommandExtension((&sshcmd.CommandContext{Transport: &defaultTransport.TransportFlag}).RegisterAddKeys, false)

	cmd.AddCommandExtension(cleancmd.RegisterCleanup, true)
	cmd.AddCommandExtension(initcmd.RegisterInit, true)

	jobs.AddJobExtension(cjobs.NewContainerExtension())
	jobs.AddJobExtension(gitjobs.NewGitExtension())
	jobs.AddJobExtension(routerjobs.NewRouterExtension())
	jobs.AddJobExtension(sshjobs.NewSshExtension())

	http.AddHttpExtension(&chttp.HttpExtension{})
	http.AddHttpExtension(&githttp.HttpExtension{})
	http.AddHttpExtension(&sshhttp.HttpExtension{})
	http.AddHttpExtension(&routerhttp.HttpExtension{})
}
