package utils

import (
	"log"
	"net"
)

var (
	Resolver addressResolver = addressResolver{}
)

type addressResolver struct {
	local   net.IP
	checked bool
}

func (resolver *addressResolver) ResolveIP(host string) (net.IP, error) {
	if host == "localhost" || host == "127.0.0.1" {
		if resolver.local != nil {
			return resolver.local, nil
		}
		if !resolver.checked {
			resolver.checked = true
			devices, err := net.Interfaces()
			if err != nil {
				return nil, err
			}
			for _, dev := range devices {
				if (dev.Flags&net.FlagUp != 0) && (dev.Flags&net.FlagLoopback == 0) {
					addrs, err := dev.Addrs()
					if err != nil {
						continue
					}
					for i := range addrs {
						if ip, ok := addrs[i].(*net.IPNet); ok {
							log.Printf("Using %v for %s", ip, host)
							resolver.local = ip.IP
							return resolver.local, nil
						}
					}
				}
			}
		}
	}
	addr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, err
	}
	return addr.IP, nil
}
