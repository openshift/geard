package network

import (
	"errors"
	"github.com/dotcloud/docker/pkg/netlink"
	"net"
)

var (
	ErrNoDefaultRoute = errors.New("no default network route found")
)

func InterfaceUp(name string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	return netlink.NetworkLinkUp(iface)
}

func SetMtu(name string, mtu int) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	return netlink.NetworkSetMTU(iface, mtu)
}

func GetDefaultMtu() (int, error) {
	routes, err := netlink.NetworkGetRoutes()
	if err != nil {
		return -1, err
	}
	for _, r := range routes {
		if r.Default {
			return r.Iface.MTU, nil
		}
	}
	return -1, ErrNoDefaultRoute
}
