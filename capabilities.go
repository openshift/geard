package libcontainer

import (
	"github.com/syndtr/gocapability/capability"
	"os"
)

var capMap = map[Capability]capability.Cap{
	CAP_SETPCAP:        capability.CAP_SETPCAP,
	CAP_SYS_MODULE:     capability.CAP_SYS_MODULE,
	CAP_SYS_RAWIO:      capability.CAP_SYS_RAWIO,
	CAP_SYS_PACCT:      capability.CAP_SYS_PACCT,
	CAP_SYS_ADMIN:      capability.CAP_SYS_ADMIN,
	CAP_SYS_NICE:       capability.CAP_SYS_NICE,
	CAP_SYS_RESOURCE:   capability.CAP_SYS_RESOURCE,
	CAP_SYS_TIME:       capability.CAP_SYS_TIME,
	CAP_SYS_TTY_CONFIG: capability.CAP_SYS_TTY_CONFIG,
	CAP_MKNOD:          capability.CAP_MKNOD,
	CAP_AUDIT_WRITE:    capability.CAP_AUDIT_WRITE,
	CAP_AUDIT_CONTROL:  capability.CAP_AUDIT_CONTROL,
	CAP_MAC_OVERRIDE:   capability.CAP_MAC_OVERRIDE,
	CAP_MAC_ADMIN:      capability.CAP_MAC_ADMIN,
}

// Drop capabilities for the current process based
// on the container's configuration
func DropCapabilities(container *Container) error {
	if drop := getCapabilities(container); len(drop) > 0 {
		c, err := capability.NewPid(os.Getpid())
		if err != nil {
			return err
		}
		c.Unset(capability.CAPS|capability.BOUNDS, drop...)

		if err := c.Apply(capability.CAPS | capability.BOUNDS); err != nil {
			return err
		}
	}
	return nil
}

func getCapabilities(container *Container) []capability.Cap {
	drop := []capability.Cap{}
	for _, c := range container.Capabilities {
		drop = append(drop, capMap[c])
	}
	return drop
}
