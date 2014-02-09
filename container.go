package libcontainer

import (
	"net"
)

type Container struct {
	ID           string       `json:"id"`
	NsPid        int          `json:"namespace_pid"`
	Command      *Command     `json:"command"`
	RootFs       string       `json:"rootfs"`
	Network      *Network     `json:"network"`
	User         string       `json:"user"`
	WorkingDir   string       `json:"working_dir"`
	Namespaces   Namespaces   `json:"namespaces"`
	Capabilities Capabilities `json:"capabilities"`
}

type Command struct {
	Args []string `json:"args"`
	Env  []string `json:"environment"`
}

type Network struct {
	IP          net.IP `json:"id"`
	IPPrefixLen int    `json:"ip_prefix_len"`
	Gateway     net.IP `json:"gateway"`
	Bridge      string `json:"bridge"`
	Mtu         int    `json:"mtu"`
}
