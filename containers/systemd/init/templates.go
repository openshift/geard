package init

import (
	"text/template"

	"github.com/openshift/geard/port"
)

type ContainerInitScript struct {
	CreateUser     bool
	ContainerUser  string
	Uid            string
	Gid            string
	Command        string
	HasVolumes     bool
	Volumes        string
	PortPairs      port.PortPairs
	UseSocketProxy bool
}

var ContainerInitTemplate = template.Must(template.New("container-init.sh").Parse(`#!/bin/bash
{{ if .CreateUser }}
groupadd -g {{.Gid}} {{.ContainerUser}}
useradd -u {{.Uid}} -g {{.Gid}} {{.ContainerUser}}
{{ else }}
old_id=$(id -u {{.ContainerUser}})
old_gid=$(id -g {{.ContainerUser}})
/usr/sbin/usermod --uid {{.Uid}} {{.ContainerUser}}
/usr/sbin/groupmod --gid {{.Gid}} {{.ContainerUser}}
for i in $(find / -uid ${old_id}); do PATH=/bin:/sbin:/usr/bin:/usr/sbin chown -R {{.Uid}} $i; done
for i in $(find / -gid ${old_gid}); do PATH=/bin:/sbin:/usr/bin:/usr/sbin chgrp -R {{.Gid}} $i; done
{{ end }}
{{ if .HasVolumes }}
chown -R {{.Uid}}:{{.Gid}} {{.Volumes}}
{{ end }}
{{ if .UseSocketProxy }}
bash -c 'LISTEN_PID=$$ exec /usr/sbin/systemd-socket-proxyd {{ range .PortPairs }}127.0.0.1:{{ .Internal }}{{ end }}' &
{{ end }}
exec su {{.ContainerUser}} -s /bin/bash -c /.container.cmd
`))

var ContainerCmdTemplate = template.Must(template.New("container-cmd.sh").Parse(`#!/bin/bash
exec {{.Command}}
`))

type OutboundNetworkIptables struct {
	// The IP address for inbound source NAT
	SourceAddr string
	// The local IP and port to connect to
	LocalAddr string
	LocalPort port.Port
	// The remote IP and port to connect to
	DestAddr string
	DestPort port.Port
}

var OutboundNetworkIptablesTemplate = template.Must(template.New("outbound_network.iptables").Parse(`
-A PREROUTING -d {{.LocalAddr}}/32 -p tcp -m tcp --dport {{.LocalPort}} -j DNAT --to-destination {{.DestAddr}}:{{.DestPort}}
-A OUTPUT -d {{.LocalAddr}}/32 -p tcp -m tcp --dport {{.LocalPort}} -j DNAT --to-destination {{.DestAddr}}:{{.DestPort}}
-A POSTROUTING -o eth0 -j SNAT --to-source {{.SourceAddr}}
`))
