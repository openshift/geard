package geard

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"
)

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
