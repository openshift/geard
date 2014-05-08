package jobs

import (
	"text/template"
)

var SliceRouterTemplate = template.Must(template.New("routerhost.slice").Parse(`[Unit]
Description=Router slice

[Slice]
CPUAccounting=yes
MemoryAccounting=yes
MemoryLimit=512M
Slice=user.slice

[Install]
WantedBy=user.service`))

var UnitRouterTemplate = template.Must(template.New("routerhost.service").Parse(`[Unit]
Description=Git host

[Install]
WantedBy=multi-user.target

[Service]
Type=simple
Slice=routerhost.slice
ExecStartPre=- /bin/sh -c '/usr/bin/docker inspect -format="Removing old geard-router" "geard-router" 2>/dev/null && /usr/bin/docker rm "geard-router"'
ExecStart=/usr/bin/docker run --name "geard-router" -v /var/lib/containers/router:/var/lib/containers/router:rw -v /etc:/host_etc:ro -a stdout -a stderr --rm "rajatchopra/geard-router"
ExecStop=/usr/bin/docker stop "geard-router"
Restart=on-failure`))
