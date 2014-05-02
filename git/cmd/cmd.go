package cmd

import (
	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/git"
	gitjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/jobs"
	sshjobs "github.com/openshift/geard/ssh/jobs"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/geard/transport"

	"github.com/spf13/cobra"
	"os"
)

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

func registerLocal(parent *cobra.Command) {
	initRepoCmd := &cobra.Command{
		Use:   "init-repo <name> [<url>]",
		Short: `(Local) Setup the environment for a git repository`,
		Long:  ``,
		Run:   initRepository,
	}
	parent.AddCommand(initRepoCmd)
}

func registerRemote(parent *cobra.Command) {
	createCmd := &cobra.Command{
		Use:   "create-repo <name> [<url>]",
		Short: "Create a new git repository",
		Run:   repoCreate,
	}
	parent.AddCommand(createCmd)
}

func repoCreate(c *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> [<clone repo url>]\n")
	}

	t := c.Flags().Lookup("transport").Value.(*transport.TransportFlag).Get()

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
		Serial: func(on Locator) jobs.Job {
			return &gitjobs.CreateRepositoryRequest{
				Id:        git.RepoIdentifier(on.(*ResourceLocator).Id),
				CloneUrl:  cloneUrl,
				RequestId: jobs.NewRequestIdentifier(),
			}
		},
		Output:    os.Stdout,
		LocalInit: LocalInitializers(systemd.Start, containers.InitializeData),
		Transport: t,
	}.StreamAndExit()
}

func initRepository(cmd *cobra.Command, args []string) {
	if len(args) < 1 || len(args) > 2 {
		Fail(1, "Valid arguments: <repo_id> [<repo_url>]\n")
	}

	repoId, err := containers.NewIdentifier(args[0])
	if err != nil {
		Fail(1, "Argument 1 must be a valid repository identifier: %s\n", err.Error())
	}

	repoUrl := ""
	if len(args) == 2 {
		repoUrl = args[1]
	}

	if err := gitjobs.InitializeRepository(git.RepoIdentifier(repoId), repoUrl); err != nil {
		Fail(2, "Unable to initialize repository %s\n", err.Error())
	}
}
