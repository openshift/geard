package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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

func CreateFileOnce(path string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if os.IsExist(err) {
		file.Close()
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func AtomicReplaceLink(from, target string) error {
	newpath := from + ".replace.tmp"
	if err := os.Link(from, newpath); err != nil {
		return err
	}
	return os.Rename(newpath, target)
}

var ErrLockTaken = errors.New("An exclusive lock already exists on the specified file.")

func OpenFileExclusive(path string, mode os.FileMode) (*os.File, bool, error) {
	exists := false
	file, errf := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, mode)
	if errf != nil {
		if !os.IsExist(errf) {
			return nil, false, errf
		}
		exists = true
		file, errf = os.OpenFile(path, os.O_CREATE|os.O_RDWR, mode)
		if errf != nil {
			return nil, false, errf
		}
	}
	if errl := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errl != nil {
		if errl == syscall.EWOULDBLOCK {
			return nil, exists, ErrLockTaken
		}
		return nil, exists, errl
	}
	return file, exists, nil
}

func CreateFileExclusive(path string, mode os.FileMode) (*os.File, error) {
	file, errf := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if errf != nil {
		return nil, errf
	}
	return file, nil
}

func CopyBinary(src string, dest string, setUid bool) error {
	var err error
	var sourceInfo os.FileInfo

	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}

	if sourceInfo, err = os.Stat(src); err != nil {
		return err
	}

	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("Cannot copy source %s", src)
	}

	if _, err = os.Stat(dest); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if err = os.Remove(dest); err != nil {
			return err
		}
	}

	var mode os.FileMode
	if setUid {
		mode = 0555 | os.ModeSetuid
	} else {
		mode = 0555
	}

	var destFile *os.File
	if destFile, err = os.Create(dest); err != nil {
		return err
	}
	defer destFile.Close()

	var sourceFile *os.File
	if sourceFile, err = os.Open(src); err != nil {
		return err
	}
	defer sourceFile.Close()

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}
	destFile.Sync()

	if err = destFile.Chmod(mode); err != nil {
		return err
	}

	return nil
}
