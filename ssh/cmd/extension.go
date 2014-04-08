package cmd

import (
	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/ssh/jobs"
	"github.com/spf13/cobra"
)

type PermissionCommandHandler interface {
	DefineFlags(cmd *cobra.Command)
	CreatePermission(cmd *cobra.Command, id string) (*jobs.KeyPermission, error)
}

var permissionHandlers map[cmd.ResourceType]PermissionCommandHandler

func AddPermissionCommand(name cmd.ResourceType, handler PermissionCommandHandler) {
	if permissionHandlers == nil {
		permissionHandlers = make(map[cmd.ResourceType]PermissionCommandHandler)
	}
	permissionHandlers[name] = handler
}

func defineFlags(cmd *cobra.Command) {
	if permissionHandlers == nil {
		return
	}
	for _, handler := range permissionHandlers {
		handler.DefineFlags(cmd)
	}
}
