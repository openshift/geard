package cmd

import (
	"github.com/spf13/cobra"
)

func RegisterRouter(parent *cobra.Command) {
	testCmd := &cobra.Command{
		Use:   "test-router",
		Short: "(Local) Test router extension.",
		Run:   test,
	}
	parent.AddCommand(testCmd)
}

func test(c *cobra.Command, args []string) {
}
