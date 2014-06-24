package init

import (
	dc "github.com/fsouza/go-dockerclient"
	"github.com/spf13/cobra"
	"os/user"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
)

var (
	initArgs     = initCommand{options: dc.CreateContainerOptions{Config: &dc.Config{}}}
	dockerSocket = "unix:///var/run/docker.sock"
)

type initCommand struct {
	isolate bool
	refresh bool
	options dc.CreateContainerOptions
}

func RegisterLocal(parent *cobra.Command) {
	initContainerCmd := &cobra.Command{
		Use:   "init-container [--isolate] [--refresh] <runArgs>",
		Short: "(Local) Setup the environment for a container",
		Long:  "Creates the appropriate container for execution",
		Run:   initArgs.initContainer,
	}
	initContainerCmd.Flags().StringVar(&initArgs.options.Name, "name", "", "The name of a container")
	initContainerCmd.Flags().BoolVar(&initArgs.isolate, "isolate", false, "Should the container be isolated")
	initContainerCmd.Flags().BoolVar(&initArgs.refresh, "refresh", true, "Should the container be recreated each restart")
	parent.AddCommand(initContainerCmd)

	linkContainerCmd := &cobra.Command{
		Use:   "link-container <name>",
		Short: "(Local) Set container network links",
		Long:  "Joins a container and ensures the most recent network links are loaded",
		Run:   linkContainer,
	}
	parent.AddCommand(linkContainerCmd)
}

func (i *initCommand) initContainer(c *cobra.Command, args []string) {
	if i.options.Name == "" {
		cmd.Fail(1, "--name is required")
	}
	id, err := containers.NewIdentifier(i.options.Name)
	if err != nil {
		cmd.Fail(1, "The identifier is not valid: %s", err)
	}
	if len(args) < 1 || len(args[0]) == 0 {
		cmd.Fail(1, "You must specify the image name as the first argument")
	}

	u, err := user.Lookup(id.LoginFor())
	if err != nil {
		if _, ok := err.(user.UnknownUserError); !ok {
			cmd.Fail(1, "Unable to lookup container user: %v", err)
		}
		if err := createUser(id); err != nil {
			cmd.Fail(1, "Unable to create container user: %v", err)
		}
		if u, err = user.Lookup(id.LoginFor()); err != nil {
			cmd.Fail(1, "Unable to lookup new container user: %v", err)
		}
	}

	i.options.Config.Image = args[0]
	i.options.Config.Cmd = args[1:]

	i.options.Config.AttachStdout = true
	i.options.Config.AttachStderr = true

	hooks := []ContainerHook{}
	// isolate is disabled
	// if i.isolate {
	// 	hook := &Isolator{User: u}
	// 	defer hook.Close()
	// 	hooks = append(hooks, hook)
	// }

	client, err := dc.NewClient(dockerSocket)
	if err != nil {
		cmd.Fail(1, "Unable to connect to Docker: %s", err)
	}
	creator := DataContainerPattern{Client: client, Hooks: hooks}
	if err := creator.Create(i.options); err != nil {
		cmd.Fail(1, err.Error())
	}
}

func linkContainer(c *cobra.Command, args []string) {
}
