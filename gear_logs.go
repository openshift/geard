package geard

import (
	"io"
	"os/exec"
)

func ProcessLogsFor(id ProvidesUnitName) (io.ReadCloser, error) {
	return ProcessLogsForUnit(id.UnitNameFor())
}

func ProcessLogsForUnit(unit string) (io.ReadCloser, error) {
	cmd := exec.Command("/usr/bin/journalctl", "--since=now", "-f", "--unit", unit)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return stdout, nil
}
