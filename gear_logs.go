package geard

import (
	"io"
	"os/exec"
)

func ProcessLogsForGear(id string) (io.ReadCloser, error) {
	cmd := exec.Command("/usr/bin/journalctl", "--since=now", "-f", "--unit", UnitNameForGear(id))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return stdout, nil
}
