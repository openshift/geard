package main

// This is a wrapper for the 'docker run' command which provides a
// $SYSTEMD_READY variable inside container which the STI scripts use to
// provide a 'ready' notification for the systemd 'notify' type of service the
// container was launched from.
//
// The $SYSTEMD_READY is a UNIX named pipe. To indicate the 'ready/failed'
// state, scripts might use:
//
// echo 1 > $SYSTEMD_READY # -> The container is 'ready'
// echo 'ERRNO=1' > $SYSTEMD_READY # -> The container failed to become 'ready'
//
// This wrapper should never be invoked directly, but only inside the systemd
// service file:
//
// [Service]
// Type=notify
// ExecStart=/data/bin/docker-notify <docker run args>
//

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/openshift/geard/cmd/docker-notify/sdnotify"
	"github.com/openshift/geard/cmd/docker-notify/tempfifo"
)

func main() {

	readyPipe, err := tempfifo.MkTempFifo("container-notify-", ".sock")

	if err != nil {
		handleError(err, "MkTempFifo")
	}

	defer tempfifo.RmTempFifo(readyPipe)

	// Might use https://github.com/dotcloud/docker/blob/master/api/client/commands.go#L1818
	// instead of '/usr/bin/docker'. In that case we can get rid of extra process.
	//
	cmd := dockerRunCmd(append(os.Args[:0], os.Args[1:]...), readyPipe)

	go waitForContainerNotification(readyPipe)

	if err := cmd.Start(); err != nil {
		handleError(err, "cmd.Start")
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println(err)
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
	}
}

func dockerRunCmd(args []string, readyPipe string) *exec.Cmd {

	dockerCmd := []string{"/usr/bin/docker", "run"}
	notifyArgs := []string{"-v", readyPipe + ":/.ready", "-e", "SYSTEMD_READY=/.ready"}

	return &exec.Cmd{
		Path:   dockerCmd[0],
		Args:   append(dockerCmd, append(notifyArgs, args...)...),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func waitForContainerNotification(readyPipe string) {
	rawMsg, err := ioutil.ReadFile(readyPipe)
	if err != nil {
		forwardReadyNotification("ERRNO=255")
	} else {
		forwardReadyNotification(strings.TrimSpace(string(rawMsg)))
	}
}

func forwardReadyNotification(message string) {
	fmt.Printf("Sending systemd notification (%s)\n", translateMessage(message))
	if err := sdnotify.SendNotify(translateMessage(message)); err != nil {
		fmt.Printf("Error delivering ready notification to systemd (%s)\n", err)
	}
}

func translateMessage(message string) string {
	switch message {
	case "1":
		return "READY=1"
	case "0":
		return "READY=0"
	default:
		return message
	}
}

func handleError(err error, message string) {
	fmt.Printf("%s: %s\n", message, err)
	os.Exit(1)
}
