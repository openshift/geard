package git

import (
	"text/template"
)

var TargetGitTemplate = template.Must(template.New("githost.target").Parse(`[Unit]
Description=Git Host target

[Install]
WantedBy=multi-user.target`))

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

[Service]
Type=simple
Slice=githost.slice
ExecStart=/bin/sh -c '/usr/bin/docker inspect -format="Removing old geard-git-host" "geard-git-host" 2>/dev/null && \
                      exec /usr/bin/docker rm "geard-git-host" ; \
                      exec /usr/bin/docker run -name "geard-git-host" -v /var/lib/containers/git:/var/lib/containers/git:rw -v /etc:/host_etc:ro -a stdout -a stderr -rm "kraman/githost"'
ExecStop=/usr/bin/docker stop "geard-git-host"
Restart=on-failure`))

type GitUserUnit struct {
	ExecutablePath string
	GitRepo        RepoIdentifier
	GitURL         string
}

var UnitGitRepoTemplate = template.Must(template.New("git-repo-xxx.service").Parse(`[Unit]
Description=Git host

[Service]
Type=oneshot
Slice=githost.slice
ExecStart={{.ExecutablePath}} init-repo "{{.GitRepo}}" "{{.GitURL}}"`))

var SliceGitRepoTemplate = template.Must(template.New("user-xxx.slice").Parse(`[Unit]
Description=Git {{.GitRepo}}

[Slice]
Slice=githost.slice

[Install]
WantedBy=user.service`))
