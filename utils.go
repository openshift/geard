package libcontainer

import (
	"os"
	"syscall"
)

func WaitOnPid(pid int) (exitcode int, err error) {
	child, err := os.FindProcess(pid)
	if err != nil {
		return -1, err
	}
	state, err := child.Wait()
	if err != nil {
		return -1, err
	}
	return getExitCode(state), nil
}

func getExitCode(state *os.ProcessState) int {
	return state.Sys().(syscall.WaitStatus).ExitStatus()
}
