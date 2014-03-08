package git

import (
	"bufio"
	"fmt"
	"github.com/smarterclayton/geard/config"
	"github.com/smarterclayton/geard/selinux"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func GenerateAuthorizedKeys(repoId RepoIdentifier, u *user.User, forceCreate bool, printToStdOut bool) error {
	var err error
	var sshKeys []string
	var destFile *os.File
	var srcFile *os.File
	var w *bufio.Writer

	sshKeys, err = filepath.Glob(path.Join(repoId.SshAccessBasePath(), "*"))
	if err != nil {
		return err
	}

	if !printToStdOut {
		os.MkdirAll(repoId.HomePath(), 0700)
		os.Mkdir(path.Join(repoId.HomePath(), ".ssh"), 0700)
		authKeysPath := repoId.AuthKeysPathFor()
		if _, err = os.Stat(authKeysPath); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			if forceCreate {
				os.Remove(authKeysPath)
			} else {
				return nil
			}
		}

		if destFile, err = os.Create(authKeysPath); err != nil {
			return err
		}
		defer destFile.Close()
		destFile.Chmod(0400)
		w = bufio.NewWriter(destFile)
	} else {
		w = bufio.NewWriter(os.Stdout)
	}

	for _, keyFile := range sshKeys {
		s, err := os.Stat(keyFile)
		if err != nil {
			continue
		}
		if s.IsDir() {
			continue
		}

		srcFile, err = os.Open(keyFile)
		defer srcFile.Close()

		readwriteRepo := strings.HasSuffix(keyFile, ".write")
		if readwriteRepo {
			w.WriteString(fmt.Sprintf("command=\"%v/bin/switchns --git\",no-agent-forwarding,no-X11-forwarding,no-port-forwarding ", config.ContainerBasePath()))
		} else {
			w.WriteString(fmt.Sprintf("command=\"%v/bin/switchns --git-ro\",no-agent-forwarding,no-X11-forwarding,no-port-forwarding ", config.ContainerBasePath()))
		}

		io.Copy(w, srcFile)
		w.WriteString("\n")
	}
	w.Flush()

	if !printToStdOut {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)

		for _, path := range []string{
			repoId.HomePath(),
			filepath.Join(repoId.HomePath(), ".ssh"),
			filepath.Join(repoId.HomePath(), ".ssh", "authorized_keys"),
		} {
			if err := os.Chown(path, uid, gid); err != nil {
				return err
			}
		}

		if err := selinux.RestoreCon(repoId.BaseHomePath(), true); err != nil {
			return err
		}
	}
	return nil
}
