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

func AddCommand(parent *cobra.Command, cmd *cobra.Command, local bool) *cobra.Command {
	parent.AddCommand(cmd)
	for i := range extensions {
		ext := &extensions[i]
		if ext.run == false && local == ext.local && ext.After == cmd.Name() {
			ext.Func(cmd.Parent())
			ext.run = true
		}
	}
	return cmd
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

type initializerHook struct {
	Func FuncInit
	when []string
	run  bool
}

// All command extensions
var initializers []initializerHook

// Register an initializer to be invoked and an optional list of conditions. If
// no conditions are passed the initializer is run whenever a command invokes
// cmd.Initialize()
func AddInitializer(init FuncInit, when ...string) {
	initializers = append(initializers, initializerHook{init, when, false})
}
