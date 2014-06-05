package cmd

import (
	"github.com/spf13/cobra"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/ssh/jobs"
)

type PermissionCommandHandler interface {
	DefineFlags(c *cobra.Command)
	CreatePermission(c *cobra.Command, id string) (*jobs.KeyPermission, error)
}

var permissionHandlers map[cmd.ResourceType]PermissionCommandHandler

func AddPermissionCommand(name cmd.ResourceType, handler PermissionCommandHandler) {
	if permissionHandlers == nil {
		permissionHandlers = make(map[cmd.ResourceType]PermissionCommandHandler)
	}
	permissionHandlers[name] = handler
}

func defineFlags(c *cobra.Command) {
	if permissionHandlers == nil {
		return
	}
	for _, handler := range permissionHandlers {
		handler.DefineFlags(c)
	}
}
