package streams

import (
	//"errors"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
)

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

func LimitWriter(w io.Writer, n int64) io.Writer { return &LimitedWriter{w, n} }

type LimitedWriter struct {
	W io.Writer // underlying writer
	N int64     // max bytes remaining
}

func (l *LimitedWriter) Write(p []byte) (n int, err error) {
	incoming := int64(len(p))
	left := l.N
	if left == 0 {
		n = int(incoming)
		return
	} else if incoming <= left {
		l.N = left - incoming
		return l.W.Write(p)
	}
	l.N = 0
	n = int(incoming)
	_, err = l.W.Write(p[:left])
	return
}

func WriteGitRepositoryArchive(w io.Writer, path string, ref GitCommitRef) error {
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
	cmd.Stderr = LimitWriter(&stderr, 20*1024)

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
