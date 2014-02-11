package geard

import (
	"errors"
	"io"
	"log"
	"os/exec"
	"time"
)

var ErrLogWriteTimeout = errors.New("gear_logs: Maximum duration exceeded, timeout")

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

func WriteLogsTo(w io.Writer, unit string, until time.Duration) error {
	cmd := exec.Command("/usr/bin/journalctl", "--since=now", "-f", "--unit", unit)
	cmd.Stdout = NewWriteFlusher(w)
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Kill()
	done := make(chan error)
	go func() {
		if err := cmd.Wait(); err != nil {
			done <- err
		}
		close(done)
	}()
	if until != 0 {
		select {
		case err := <-done:
			log.Print("gear_logs: Error when writing to log: ", err)
			return err
		case <-time.After(until):
			return ErrLogWriteTimeout
		}
	}
	return nil
}
