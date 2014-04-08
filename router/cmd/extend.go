// +build !disable_router

package cmd

import (
	. "github.com/openshift/geard/cmd"
	"github.com/spf13/cobra"
)

func init() {
	AddCommandExtension(func(parent *cobra.Command) {
		testCmd := &cobra.Command{
			Use:   "test-router",
			Short: "(Local) Test router extension.",
			Run:   test,
		}
		parent.AddCommand(testCmd)
	}, true)
}
