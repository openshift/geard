package git

import (
	"fmt"
	"github.com/smarterclayton/geard/gears"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/systemd"
	"github.com/smarterclayton/geard/utils"
	"github.com/smarterclayton/go-systemd/dbus"
	"io"
	"log"
	"os"
	"time"
)

var (
	ErrRepositoryAlreadyExists = jobs.SimpleJobError{jobs.JobResponseAlreadyExists, "A repository with this identifier already exists."}
	ErrSubscribeToUnit         = jobs.SimpleJobError{jobs.JobResponseError, "Unable to watch for the completion of this action."}
	ErrRepositoryCreateFailed  = jobs.SimpleJobError{jobs.JobResponseError, "Unable to create the repository."}
)

type CreateRepositoryRequest struct {
	RepositoryId gears.Identifier
	Image        string
	CloneUrl     string
}

const repositoryOwnerUid = 1001
const repositoryOwnerGid = 1001

func (j CreateRepositoryRequest) Execute(resp jobs.JobResponse) {
	repositoryPath := j.RepositoryId.RepositoryPathFor()
	unitName := fmt.Sprintf("job-repo-create-%s.service", j.RepositoryId)
	cloneUrl := j.CloneUrl

	if err := os.Mkdir(repositoryPath, 0770); err != nil {
		if os.IsExist(err) {
			resp.Failure(ErrRepositoryAlreadyExists)
		} else {
			log.Printf("job_create_repository: make repository dir: %+v", err)
			resp.Failure(ErrRepositoryCreateFailed)
		}
		return
	}
	if err := os.Chown(repositoryPath, repositoryOwnerUid, repositoryOwnerGid); err != nil {
		log.Printf("job_create_repository: Unable to set owner for repository path %s: %s", repositoryPath, err.Error())
	}

	conn, errc := systemd.NewConnection()
	if errc != nil {
		log.Print("job_create_repository: systemd: ", errc)
		resp.Failure(ErrSubscribeToUnit)
		return
	}
	//defer conn.Close()
	if err := conn.Subscribe(); err != nil {
		log.Print("job_create_repository: subscribe: ", err)
		resp.Failure(ErrSubscribeToUnit)
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

	stdout, err := gears.ProcessLogsForUnit(unitName)
	if err != nil {
		stdout = utils.EmptyReader
		log.Printf("job_create_repository: Unable to fetch build logs: %+v", err)
	}
	defer stdout.Close()

	// Start unit after subscription and logging has begun, since
	// we don't want to miss extremely fast events
	status, err := systemd.Connection().StartTransientUnit(
		unitName,
		"fail",
		dbus.PropExecStart([]string{
			"/usr/bin/docker", "run",
			"-rm",
			"-a", "stderr", "-a", "stdout",
			"-u", "git", "-v", repositoryPath + ":" + "/home/git/repo:rw",
			j.Image,
			cloneUrl,
		}, true),
		dbus.PropDescription(fmt.Sprintf("Create a repository (%s)", repositoryPath)),
		dbus.PropRemainAfterExit(true),
		dbus.PropSlice("gear.slice"),
	)
	if err != nil {
		log.Printf("job_create_repository: Could not start transient unit: %s", systemd.SprintSystemdError(err))
		resp.Failure(ErrRepositoryCreateFailed)
		return
	} else if status != "done" {
		log.Printf("job_create_repository: Unit did not return 'done'")
		resp.Failure(ErrRepositoryCreateFailed)
		return
	}

	w := resp.SuccessWithWrite(jobs.JobResponseAccepted, true)
	go io.Copy(w, stdout)

wait:
	for {
		select {
		case c := <-changes:
			if changed, ok := c[unitName]; ok {
				if changed.SubState != "running" {
					fmt.Fprintf(w, "Repository completed (%s)\n", changed.SubState)
					break wait
				}
			}
		case err := <-errch:
			fmt.Fprintf(w, "Error %+v\n", err)
		case <-time.After(10 * time.Second):
			log.Print("job_create_repository:", "timeout")
			break wait
		}
	}

	stdout.Close()
}
