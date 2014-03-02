package utils

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
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
