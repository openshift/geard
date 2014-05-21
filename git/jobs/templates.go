package jobs

import (
	"text/template"
)

var SliceGitTemplate = template.Must(template.New("githost.slice").Parse(`[Unit]
Description=Git Host slice

[Slice]
CPUAccounting=yes
MemoryAccounting=yes
MemoryLimit=512M
Slice=user.slice

[Install]
WantedBy=user.service`))

var UnitGitHostTemplate = template.Must(template.New("githost.service").Parse(`[Unit]
Description=Git host

[Install]
WantedBy=multi-user.target

[Service]
Type=simple
Slice=githost.slice
ExecStartPre=- /bin/sh -c '/usr/bin/docker inspect -format="Removing old geard-githost" "geard-githost" 2>/dev/null && /usr/bin/docker rm "geard-githost"'
ExecStart=/usr/bin/docker run --name "geard-githost" -v /var/lib/containers/git:/var/lib/containers/git:rw -v /etc:/host_etc:ro -a stdout -a stderr --rm "ccoleman/geard-githost"
ExecStop=/usr/bin/docker stop "geard-githost"
Restart=on-failure`))
