package jobs

import (
	"fmt"
	"github.com/smarterclayton/go-systemd/dbus"
	"github.com/smarterclayton/geard/gear"	
	"github.com/smarterclayton/geard/systemd"		
	"github.com/smarterclayton/geard/utils"			
	"io"
	"log"
	"os"
	"strconv"
	"time"
)

type CreateRepositoryJobRequest struct {
	JobResponse
	JobRequest
	RepositoryId gear.Identifier
	UserId       string
	Image        string
	CloneUrl     string
}

const repositoryOwnerUid = 1001
const repositoryOwnerGid = 1001

func (j *CreateRepositoryJobRequest) Execute() {
	repositoryPath := j.RepositoryId.RepositoryPathFor()
	unitName := j.RepositoryId.UnitNameForJob()
	cloneUrl := j.CloneUrl

	if err := os.Mkdir(repositoryPath, 0770); err != nil {
		if os.IsExist(err) {
			j.Failure(ErrRepositoryAlreadyExists)
		} else {
			log.Printf("job_create_repository: make repository dir: %+v", err)
			j.Failure(ErrRepositoryCreateFailed)
		}
		return
	}
	if err := os.Chown(repositoryPath, repositoryOwnerUid, repositoryOwnerGid); err != nil {
		log.Printf("job_create_repository: Unable to set owner for repository path %s: %s", repositoryPath, err.Error())
	}

	conn, errc := systemd.NewSystemdConnection()
	if errc != nil {
		log.Print("job_create_repository: systemd: ", errc)
		j.Failure(ErrSubscribeToUnit)
		return
	}
	//defer conn.Close()
	if err := conn.Subscribe(); err != nil {
		log.Print("job_create_repository: subscribe: ", err)
		j.Failure(ErrSubscribeToUnit)
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

	stdout, err := gear.ProcessLogsForUnit(unitName)
	if err != nil {
		stdout = utils.EmptyReader
		log.Printf("job_create_repository: Unable to fetch build logs: %+v", err)
	}
	defer stdout.Close()

	// Start unit after subscription and logging has begun, since
	// we don't want to miss extremely fast events
	status, err := systemd.SystemdConnection().StartTransientUnit(
		unitName,
		"fail",
		dbus.PropExecStart([]string{
			"/usr/bin/docker", "run",
			"-rm",
			"-a", "stderr", "-a", "stdout",
			"-u", "\"" + strconv.Itoa(repositoryOwnerUid) + "\"", "-v", repositoryPath + ":" + "/home/git/repo:rw",
			j.Image,
			cloneUrl,
		}, true),
		dbus.PropDescription(fmt.Sprintf("Create a repository (%s)", repositoryPath)),
		dbus.PropRemainAfterExit(true),
		dbus.PropSlice("gear.slice"),
	)
	if err != nil {
		log.Printf("job_create_repository: Could not start transient unit: %s", systemd.SprintSystemdError(err))
		j.Failure(ErrRepositoryCreateFailed)
		return
	} else if status != "done" {
		log.Printf("job_create_repository: Unit did not return 'done'")
		j.Failure(ErrRepositoryCreateFailed)
		return
	}

	w := j.SuccessWithWrite(JobResponseAccepted, true)
	go io.Copy(w, stdout)

wait:
	for {
		select {
		case c := <-changes:
			if changed, ok := c[unitName]; ok {
				if changed.SubState != "running" {
					fmt.Fprintf(w, "Repository completed\n", changed.SubState)
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
