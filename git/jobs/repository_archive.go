package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"

	"github.com/openshift/geard/git"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/utils"
)

const ContentTypeGitArchive = "gitarchive"

type GitArchiveContentRequest struct {
	RepositoryId git.RepoIdentifier
	Ref          GitCommitRef
}

func (j GitArchiveContentRequest) Execute(resp jobs.Response) {
	w := resp.SuccessWithWrite(jobs.ResponseOk, false, false)
	if err := writeGitRepositoryArchive(w, j.RepositoryId.RepositoryPathFor(), j.Ref); err != nil {
		log.Printf("job_content: Invalid git repository stream: %v", err)
	}
}

type GitCommitRef string

const EmptyGitCommitRef = GitCommitRef("")
const InvalidGitCommitRef = GitCommitRef("")

var allowedGitCommitRef = regexp.MustCompile("\\A[a-zA-Z0-9_\\-]+\\z")

func NewGitCommitRef(s string) (GitCommitRef, error) {
	switch {
	case s == "":
		return EmptyGitCommitRef, nil
	case !allowedGitCommitRef.MatchString(s):
		return InvalidGitCommitRef, errors.New("Git ref must match " + allowedGitCommitRef.String())
	}
	return GitCommitRef(s), nil
}

type Waiter interface {
	Wait() error
}

func writeGitRepositoryArchive(w io.Writer, path string, ref GitCommitRef) error {
	var cmd *exec.Cmd
	// TODO: Stream as tar with gzip
	if ref == EmptyGitCommitRef {
		cmd = exec.Command("/usr/bin/git", "archive", "--format", "zip", "master")
	} else {
		cmd = exec.Command("/usr/bin/git", "archive", "--format", "zip", string(ref))
	}
	cmd.Env = []string{}
	cmd.Dir = path
	var stderr bytes.Buffer
	cmd.Stderr = utils.LimitWriter(&stderr, 20*1024)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	io.Copy(w, stdout)
	if err := cmd.Wait(); err != nil {
		return errors.New(fmt.Sprintf("Failed to archive repository: %s\n", err.Error()) + stderr.String())
	}
	return nil
}
