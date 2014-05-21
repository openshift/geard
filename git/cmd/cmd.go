package cmd

import (
	"github.com/spf13/cobra"
	"os"

	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/git"
	gitjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/jobs"
	sshcmd "github.com/openshift/geard/ssh/cmd"
	sshjobs "github.com/openshift/geard/ssh/jobs"
	"github.com/openshift/geard/transport"
)

func init() {
	sshcmd.AddPermissionCommand(git.ResourceTypeRepository, &handler)
}

var handler permissionHandler

// Implements the default container permission serialization
type permissionHandler struct {
	writeAccess bool
}

func (c *permissionHandler) CreatePermission(cmd *cobra.Command, id string) (*sshjobs.KeyPermission, error) {
	return sshjobs.NewKeyPermission(git.RepositoryPermissionType, &git.RepositoryPermission{id, c.writeAccess})
}
func (c *permissionHandler) DefineFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&c.writeAccess, "write", false, "Enable write access for the selected repositories")
	cmd.Long += "\n\nFor Git repositories, pass the --write flag to grant write access."
}

// Repository commands requires a transport object
type Command struct {
	Transport *transport.TransportFlag
}

func (e *Command) RegisterCreateRepo(parent *cobra.Command) {
	createCmd := &cobra.Command{
		Use:   "create-repo <name> [<url>]",
		Short: "Create a new git repository",
		Run:   e.repoCreate,
	}
	parent.AddCommand(createCmd)
}

func (e *Command) repoCreate(c *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> [<clone repo url>]\n")
	}

	t := e.Transport.Get()

	id, err := NewResourceLocator(t, git.ResourceTypeRepository, args[0])
	if err != nil {
		Fail(1, "You must pass one valid repository name: %s\n", err.Error())
	}
	if id.(*ResourceLocator).Type != git.ResourceTypeRepository {
		Fail(1, "You must pass one valid repository name: %s\n", err.Error())
	}

	cloneUrl := ""
	if len(args) == 2 {
		cloneUrl = args[1]
	}

	Executor{
		On: Locators{id},
		Serial: func(on Locator) JobRequest {
			return &gitjobs.CreateRepositoryRequest{
				Id:        git.RepoIdentifier(on.(*ResourceLocator).Id),
				CloneUrl:  cloneUrl,
				RequestId: jobs.NewRequestIdentifier(),
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}
