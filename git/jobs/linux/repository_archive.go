package linux

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"

	. "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/utils"
)

type archiveRepository struct {
	*GitArchiveContentRequest
}

func (j archiveRepository) Execute(resp jobs.Response) {
	w := resp.SuccessWithWrite(jobs.ResponseOk, false, false)
	if err := writeGitRepositoryArchive(w, j.RepositoryId.RepositoryPathFor(), j.Ref); err != nil {
		log.Printf("job_content: Invalid git repository stream: %v", err)
	}
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
