package geard

import (
	"fmt"
	"github.com/smarterclayton/go-systemd/dbus"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type createRepositoryJobRequest struct {
	jobRequest
	RepositoryId GearIdentifier
	UserId       string
	Image        string
	Output       io.Writer
}

const repositoryOwnerUid = 1001
const repositoryOwnerGid = 1001

func (j *createRepositoryJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Creating repository %s ... \n", j.RepositoryId)

	repositoryPath := j.RepositoryId.RepositoryPathFor()
	unitName := j.RepositoryId.UnitNameForJob()

	if err := os.Mkdir(repositoryPath, 0770); err != nil {
		fmt.Fprintf(j.Output, "Unable to create %s: %s", repositoryPath, err.Error())
		return
	}
	if err := os.Chown(repositoryPath, repositoryOwnerUid, repositoryOwnerGid); err != nil {
		log.Printf("job_create_repository: Unable to set owner for repository path %s: %s", repositoryPath, err.Error())
	}

	stdout, err := ProcessLogsForUnit(unitName)
	if err != nil {
		stdout = emptyReader
		fmt.Fprintf(j.Output, "Unable to fetch build logs: %s\n", err.Error())
	}
	defer stdout.Close()
	go io.Copy(j.Output, stdout)

	conn, errc := NewSystemdConnection()
	if errc != nil {
		log.Print("job_create_repository:", errc)
		fmt.Fprintf(j.Output, "Unable to watch start status", errc)
		return
	}
	//defer conn.Close()
	if err := conn.Subscribe(); err != nil {
		log.Print("job_create_repository:", err)
		fmt.Fprintf(j.Output, "Unable to watch start status", errc)
		return
	}
	defer conn.Unsubscribe()

	// make subscription global for efficiency
	changes, errch := conn.SubscribeUnitsCustom(1*time.Second, 2,
		func(s1 *dbus.UnitStatus, s2 *dbus.UnitStatus) bool {
			return true
		},
		func(unit string) bool {
			return unit != unitName
		})

	// Start unit after subscription and logging has begun, since
	// we don't want to miss extremely fast events
	status, err := SystemdConnection().StartTransientUnit(
		unitName,
		"fail",
		dbus.PropExecStart([]string{
			"/usr/bin/docker", "run",
			"-rm",
			"-a", "stderr", "-a", "stdout",
			"-u", "\"" + strconv.Itoa(repositoryOwnerUid) + "\"", "-v", repositoryPath + ":" + "/home/git/repo:rw",
			j.Image,
		}, true),
		dbus.PropRemainAfterExit(true),
	)
	if err != nil {
		fmt.Fprintf(j.Output, "Could not create repository %s\n(%s)", err.Error(), SprintSystemdError(err))
	} else if status != "done" {
		fmt.Fprintf(j.Output, "Repository not created successfully: %s\n", status)
	} else {
		fmt.Fprintf(j.Output, "\nRepository %s is being created\n", j.RepositoryId)
	}

	if flusher, ok := j.Output.(http.Flusher); ok {
		flusher.Flush()
	}

	for {
		select {
		case c := <-changes:
			if changed, ok := c[unitName]; ok {
				if changed.SubState != "running" {
					fmt.Fprintf(j.Output, "Repository completed\n", changed.SubState)
					goto done
				}
			}
		case err := <-errch:
			fmt.Fprintf(j.Output, "Error %+v\n", err)
		case <-time.After(10 * time.Second):
			log.Print("job_create_repository:", "timeout")
			goto done
		}
	}
done:
	stdout.Close()
}
