package geard

import (
	"text/template"
)

type containerUnit struct {
	Gear     Identifier
	Image    string
	PortSpec string
}

var containerUnitTemplate = template.Must(template.New("unit.service").Parse(`
[Unit]
Description=Gear container {{.Gear}}

[Service]
Type=simple
ExecStart=/usr/bin/docker run -a stdout -a stderr {{.PortSpec}} -rm "{{.Image}}"

[Install]
WantedBy=multi-user.target
`))
