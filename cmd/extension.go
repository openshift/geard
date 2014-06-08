package cmd

import (
	"github.com/spf13/cobra"
)

// Register flags and commands underneath a parent Command
type CommandRegistration func(parent *cobra.Command)

type commandHook struct {
	Func  CommandRegistration
	After string
	local bool
	run   bool
}

// All command extensions
var extensions []commandHook

// Register an extension to this server during init() or startup
func AddCommandExtension(ext CommandRegistration, local bool) {
	extensions = append(extensions, commandHook{ext, "", local, false})
}

func AddCommand(parent *cobra.Command, c *cobra.Command, local bool) *cobra.Command {
	parent.AddCommand(c)
	for i := range extensions {
		ext := &extensions[i]
		if ext.run == false && local == ext.local && ext.After == c.Name() {
			ext.Func(c.Parent())
			ext.run = true
		}
	}
	return c
}

func ExtendCommands(parent *cobra.Command, local bool) {
	for i := range extensions {
		ext := &extensions[i]
		if ext.run == false && local == ext.local {
			ext.Func(parent)
			ext.run = true
		}
	}
}
