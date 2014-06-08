package cmd

import (
	"github.com/spf13/cobra"
	"os"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/git"
	gitjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/jobs"
	sshjobs "github.com/openshift/geard/ssh/jobs"
	"github.com/openshift/geard/transport"
)

// Implements the default container permission serialization
type PermissionCommandContext struct {
	writeAccess bool
}

func (ctx *PermissionCommandContext) CreatePermission(c *cobra.Command, id string) (*sshjobs.KeyPermission, error) {
	return sshjobs.NewKeyPermission(git.RepositoryPermissionType, &git.RepositoryPermission{id, ctx.writeAccess})
}
func (ctx *PermissionCommandContext) DefineFlags(c *cobra.Command) {
	c.Flags().BoolVar(&ctx.writeAccess, "write", false, "Enable write access for the selected repositories")
	c.Long += "\n\nFor Git repositories, pass the --write flag to grant write access."
}

// Repository commands requires a transport object
type CommandContext struct {
	Transport *transport.TransportFlag
}

func (ctx *CommandContext) RegisterCreateRepo(parent *cobra.Command) {
	createCmd := &cobra.Command{
		Use:   "create-repo <name> [<url>]",
		Short: "Create a new git repository",
		Run:   ctx.repoCreate,
	}
	parent.AddCommand(createCmd)
}

func (ctx *CommandContext) repoCreate(c *cobra.Command, args []string) {
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> [<clone repo url>]\n")
	}

	t := ctx.Transport.Get()

	id, err := cmd.NewResourceLocator(t, git.ResourceTypeRepository, args[0])
	if err != nil {
		cmd.Fail(1, "You must pass one valid repository name: %s\n", err.Error())
	}
	if id.(*cmd.ResourceLocator).Type != git.ResourceTypeRepository {
		cmd.Fail(1, "You must pass one valid repository name: %s\n", err.Error())
	}

	cloneUrl := ""
	if len(args) == 2 {
		cloneUrl = args[1]
	}

	cmd.Executor{
		On: cmd.Locators{id},
		Serial: func(on cmd.Locator) cmd.JobRequest {
			return &gitjobs.CreateRepositoryRequest{
				Id:        git.RepoIdentifier(on.(*cmd.ResourceLocator).Id),
				CloneUrl:  cloneUrl,
				RequestId: jobs.NewRequestIdentifier(),
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}
