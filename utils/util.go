package utils

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var EmptyReader = ioutil.NopCloser(bytes.NewReader([]byte{}))

type flushable interface {
	io.Writer
	http.Flusher
}

type writeFlusher struct {
	flushable
}

func (f writeFlusher) Write(p []byte) (n int, err error) {
	n, err = f.flushable.Write(p)
	if n != 0 || err == nil {
		f.flushable.Flush()
	}
	return
}

/*
 * Return an io.Writer that will flush after every write.
 */
func NewWriteFlusher(w io.Writer) io.Writer {
	if f, ok := w.(flushable); ok {
		return writeFlusher{f}
	}
	return w
}

func TakePrefix(s string, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

func TakeSegment(path string) (string, string, bool) {
	segments := strings.SplitN(path, "/", 2)
	if len(segments) > 1 {
		if segments[0] == "/" {
			return TakeSegment(segments[1])
		}
		return segments[0], segments[1], true
	}
	return segments[0], "", (segments[0] != "")
}

var ErrContentMismatch = errors.New("File content does not match expected value")

func AtomicWriteToContentPath(path string, mode os.FileMode, value []byte) error {
	file, errf := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	defer file.Close()
	if os.IsExist(errf) {
		content, errr := ioutil.ReadFile(path)
		if errr == nil {
			if string(value) == string(content) {
				return nil
			} else {
				return ErrContentMismatch
			}
		}
		return errf
	} else if errf != nil {
		return errf
	} else {
		if _, err := file.Write(value); err != nil {
			os.Remove(path)
			return err
		}
	}
	return nil
}

func AtomicReplaceLink(from, target string) error {
	newpath := from + ".replace.tmp"
	if err := os.Link(from, newpath); err != nil {
		return err
	}
	return os.Rename(newpath, target)
}

var ErrLockTaken = errors.New("An exclusive lock already exists on the specified file.")

func OpenFileExclusive(path string, mode os.FileMode) (*os.File, error) {
	file, errf := os.OpenFile(path, os.O_CREATE|os.O_RDWR, mode)
	if errf != nil {
		return nil, errf
	}
	if errl := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errl != nil {
		if errl == syscall.EWOULDBLOCK {
			return nil, ErrLockTaken
		}
		return nil, errl
	}
	return file, nil
}

func CreateFileExclusive(path string, mode os.FileMode) (*os.File, error) {
	file, errf := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if errf != nil {
		return nil, errf
	}
	return file, nil
}

func IsolateContentPathWithPerm(base, id, suffix string, perm os.FileMode) string {
	var path string
	if suffix == "" {
		path = filepath.Join(base, id[0:2])
		suffix = id
	} else {
		path = filepath.Join(base, id[0:2], id)
	}
	// fail silently, require startup to set paths, let consumers
	// handle directory not found errors
	os.MkdirAll(path, perm)

	return filepath.Join(path, suffix)
}

func IsolateContentPath(base, id, suffix string) string {
	return IsolateContentPathWithPerm(base, id, suffix, 0770)
}
