package cmd

import (
	cmd "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/git"
	"github.com/openshift/geard/git/http"
	gearjobs "github.com/openshift/geard/jobs"
	"github.com/spf13/cobra"

	"os"
)

func LoadCommand(gearCmd *cobra.Command) {
	createCmd := &cobra.Command{
		Use:   "create-repo",
		Short: "Create a new git repository",
		Run:   repoCreate,
	}

	gearCmd.AddCommand(createCmd)
}

func repoCreate(c *cobra.Command, args []string) {
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> [<clone repo url>]\n")
	}

	id, err := cmd.NewGenericLocator(cmd.ResourceTypeRepository, args[0])
	if err != nil {
		cmd.Fail(1, "You must pass one valid repository name: %s\n", err.Error())
	}

	if id.ResourceType() != cmd.ResourceTypeRepository {
		cmd.Fail(1, "You must pass one valid repository name: %s\n", err.Error())
	}

	cmd.Executor{
		On: cmd.Locators{id},
		Serial: func(on cmd.Locator) gearjobs.Job {
			var req http.HttpCreateRepositoryRequest
			req = http.HttpCreateRepositoryRequest{}
			req.Id = git.RepoIdentifier(on.(cmd.ResourceLocator).Identifier())

			return &req
		},
		Output:    os.Stdout,
		LocalInit: containers.InitializeData,
	}.StreamAndExit()
}
