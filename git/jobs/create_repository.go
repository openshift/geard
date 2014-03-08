package jobs

import (
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/git"
	jobs "github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/selinux"
	"github.com/smarterclayton/geard/systemd"
	"github.com/smarterclayton/geard/utils"
	"github.com/smarterclayton/go-systemd/dbus"
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
	RepositoryId git.RepoIdentifier
	CloneUrl     string
}

func (j CreateRepositoryRequest) Execute(resp jobs.JobResponse) {
	path := j.RepositoryId.UnitPathFor()
	unitName := j.RepositoryId.UnitNameFor()
	var status string
	var err error
	unit, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)

	if os.IsExist(err) {
		resp.Failure(ErrRepositoryAlreadyExists)
		return
	} else if err != nil {
		log.Printf("job_create_repository: make repository dir: %+v", err)
		resp.Failure(ErrRepositoryCreateFailed)
		return
	}

	args := git.GitUserUnit{
		GitRepo:        j.RepositoryId,
		ExecutablePath: filepath.Join(config.ContainerBasePath(), "bin", "gear"),
		GitURL:         j.CloneUrl,
	}

	if err := git.UnitGitRepoTemplate.Execute(unit, args); err != nil {
		log.Printf("job_create_repository: Unable to write %s %s: %v", "unit", j.RepositoryId, err)
		resp.Failure(ErrRepositoryCreateFailed)
		return
	}
	if errc := unit.Close(); errc != nil {
		log.Printf("job_create_repository: Unable to close target %s %s: %v", "unit", j.RepositoryId, errc)
		resp.Failure(ErrRepositoryCreateFailed)
		return
	}

	conn, errc := systemd.NewConnection()
	if errc != nil {
		log.Print("job_create_repository:", errc)
		return
	}

	if err := conn.Subscribe(); err != nil {
		log.Print("job_create_repository:", err)
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
		log.Printf("job_create_repository: Unable to fetch build logs: %+v", err)
	}
	defer stdout.Close()

	w := resp.SuccessWithWrite(jobs.JobResponseAccepted, true, false)
	go io.Copy(w, stdout)

	status, err = systemd.StartAndEnableUnit(conn, unitName, path, "fail")
	if err != nil {
		log.Printf("job_create_repository: Could not start unit: %s", systemd.SprintSystemdError(err))
		fmt.Fprintf(w, "Repository created failed\n")
	} else if status != "done" {
		log.Printf("job_create_repository: Unit did not return 'done'")
		fmt.Fprintf(w, "Repository created failed\n")
	}

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
		case <-time.After(25 * time.Second):
			log.Print("job_create_repository:", "timeout")
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
	if err = os.Chown(repositoryId.RepositoryPathFor(), uid, gid); err != nil {
		return err
	}

	switchns := filepath.Join(config.ContainerBasePath(), "bin", "switchns")
	cmd := exec.Command(switchns, "geard-git-host", "/git/init-repo", repositoryId.RepositoryPathFor(), u.Uid, u.Gid, repositoryURL)
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
	u, err := user.Lookup(repositoryId.LoginFor())
	if err != nil {
		return err
	}

	sliceName := fmt.Sprintf("user-%v", u.Uid)
	return systemd.InitializeSystemdFile(systemd.SliceType, sliceName, git.SliceGitRepoTemplate, git.GitUserUnit{ExecutablePath: "", GitRepo: repositoryId, GitURL: ""})
}
