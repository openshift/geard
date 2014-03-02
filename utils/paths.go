package utils

import (
	"os"
	"path/filepath"
	"strings"
)

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
