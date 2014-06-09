/*
The gear 'clean' extension.
*/
package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"

	"github.com/openshift/geard/cleanup"
)

var (
	dryRun bool
	repair bool
)

func RegisterCleanup(parent *cobra.Command) {
	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "(local) Perform housekeeping tasks on geard directories",
		Long:  "Perform various tasks to clean up the state, images, directories and other resources.",
		Run:   clean,
	}
	cleanCmd.Flags().BoolVarP(&dryRun, "dry-run", "", false, "List the cleanups, but do not execute.")
	cleanCmd.Flags().BoolVarP(&repair, "repair", "", false, "Perform potentially unrecoverable cleanups.")
	parent.AddCommand(cleanCmd)
}

func clean(c *cobra.Command, args []string) {
	logInfo := log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	logError := log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime)

	cleanup.Clean(&cleanup.CleanerContext{DryRun: dryRun, Repair: repair, LogInfo: logInfo, LogError: logError})
}
