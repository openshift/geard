package jobs

import (
	"fmt"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/git"
	jobs "github.com/openshift/geard/jobs"
	"github.com/openshift/geard/selinux"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/geard/utils"
	"github.com/openshift/go-systemd/dbus"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
)

var (
	ErrRepositoryAlreadyExists = jobs.SimpleJobError{jobs.JobResponseAlreadyExists, "A repository with this identifier already exists."}
	ErrSubscribeToUnit         = jobs.SimpleJobError{jobs.JobResponseError, "Unable to watch for the completion of this action."}
	ErrRepositoryCreateFailed  = jobs.SimpleJobError{jobs.JobResponseError, "Unable to create the repository."}
)

type CreateRepositoryRequest struct {
	Id        git.RepoIdentifier
	CloneUrl  string
	RequestId jobs.RequestIdentifier
}

func (j CreateRepositoryRequest) Execute(resp jobs.JobResponse) {
	unitName := fmt.Sprintf("job-create-repo-%s.service", j.RequestId.String())
	path := j.Id.HomePath()

	if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
		resp.Failure(ErrRepositoryAlreadyExists)
		return
	}

	conn, errc := systemd.NewConnection()
	if errc != nil {
		log.Print("create_repository:", errc)
		return
	}

	if err := conn.Subscribe(); err != nil {
		log.Print("create_repository:", err)
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

	stdout, err := systemd.ProcessLogsForUnit(unitName)
	if err != nil {
		stdout = utils.EmptyReader
		log.Printf("create_repository: Unable to fetch build logs: %+v", err)
	}
	defer stdout.Close()

	startCmd := []string{
		filepath.Join(config.ContainerBasePath(), "bin", "gear"),
		"init-repo",
		string(j.Id),
	}
	if j.CloneUrl != "" {
		startCmd = append(startCmd, j.CloneUrl)
	}

	status, err := conn.StartTransientUnit(
		unitName,
		"fail",
		dbus.PropExecStart(startCmd, true),
		dbus.PropDescription(fmt.Sprintf("Create repository %s", j.Id)),
		dbus.PropRemainAfterExit(true),
		dbus.PropSlice("githost.slice"),
	)
	if err != nil {
		log.Printf("create_repository: Could not start unit %s: %s", unitName, systemd.SprintSystemdError(err))
		resp.Failure(ErrRepositoryCreateFailed)
		return
	} else if status != "done" {
		log.Printf("create_repository: Unit did not return 'done'")
		resp.Failure(ErrRepositoryCreateFailed)
		return
	}

	w := resp.SuccessWithWrite(jobs.JobResponseAccepted, true, false)
	go io.Copy(w, stdout)

wait:
	for {
		select {
		case c := <-changes:
			if changed, ok := c[unitName]; ok {
				if changed.SubState != "running" {
					fmt.Fprintf(w, "Repository created succesfully\n")
					break wait
				}
			}
		case err := <-errch:
			fmt.Fprintf(w, "Error %+v\n", err)
		case <-time.After(10 * time.Second):
			log.Print("create_repository:", "timeout")
			break wait
		}
	}

	stdout.Close()
}

func InitializeRepository(repositoryId git.RepoIdentifier, repositoryURL string) error {
	var err error
	if _, err = user.Lookup(repositoryId.LoginFor()); err != nil {
		if _, ok := err.(user.UnknownUserError); !ok {
			return err
		}
		if err = createUser(repositoryId); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(repositoryId.HomePath(), 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(repositoryId.RepositoryPathFor(), 0700); err != nil {
		return err
	}

	var u *user.User
	if u, err = user.Lookup(repositoryId.LoginFor()); err != nil {
		return err
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	
	if err = os.Chown(repositoryId.HomePath(), uid, gid); err != nil {
		return err
	}

	if err = os.Chown(repositoryId.RepositoryPathFor(), uid, gid); err != nil {
		return err
	}

	switchns := filepath.Join(config.ContainerBasePath(), "bin", "switchns")
	cmd := exec.Command(switchns, "--container=geard-githost", "--", "/git/init-repo", repositoryId.RepositoryPathFor(), u.Uid, u.Gid, repositoryURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	if err := selinux.RestoreCon(repositoryId.RepositoryPathFor(), true); err != nil {
		return err
	}
	return nil
}

func createUser(repositoryId git.RepoIdentifier) error {
	cmd := exec.Command("/usr/sbin/useradd", repositoryId.LoginFor(), "-m", "-d", repositoryId.HomePath(), "-c", "Repository user")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(out)
		return err
	}
	selinux.RestoreCon(repositoryId.HomePath(), true)
	return nil
}
