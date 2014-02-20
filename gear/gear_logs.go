package gear

import (
	"errors"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
	"github.com/smarterclayton/geard/systemd"
)

var ErrLogWriteTimeout = errors.New("gear_logs: Maximum duration exceeded, timeout")

func ProcessLogsFor(id systemd.ProvidesUnitName) (io.ReadCloser, error) {
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
	stdout, errp := cmd.StderrPipe()
	if errp != nil {
		return errp
	}
	if err := cmd.Start(); err != nil {
		stdout.Close()
		return err
	}

	wg := sync.WaitGroup{}
	outch := make(chan error, 1)
	go func() {
		wg.Add(1)
		_, err := io.Copy(w, stdout)
		outch <- err
		wg.Done()
	}()
	prcch := make(chan error, 1)
	go func() {
		wg.Add(1)
		err := cmd.Wait()
		prcch <- err
		wg.Done()
	}()

	if until == 0 {
		until = 5 * time.Second
	}

	var err error
	select {
	case err = <-prcch:
		if err != nil {
			log.Print("gear_logs: Process exited unexpectedly: ", err)
		}
	case err = <-outch:
		if err != nil {
			log.Print("gear_logs: Output closed before process exited: ", err)
		} else {
			log.Print("gear_logs: Write completed")
		}
	case <-time.After(until):
		log.Print("gear_logs: Timeout")
		err = ErrLogWriteTimeout
	}

	stdout.Close()
	cmd.Process.Kill()
	wg.Wait()

	return nil
}
