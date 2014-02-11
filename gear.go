package geard

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Identifier string

var InvalidIdentifier = Identifier("")
var allowedIdentifier = regexp.MustCompile("\\A[a-fA-F0-9]{4,32}\\z")

func NewIdentifier(s string) (Identifier, error) {
	switch {
	case s == "":
		return InvalidIdentifier, errors.New("Gear identifier may not be empty")
	case !allowedIdentifier.MatchString(s):
		return InvalidIdentifier, errors.New("Gear identifier must match " + allowedIdentifier.String())
	}
	return Identifier(s), nil
}

func (g Identifier) UnitPathFor() string {
	return filepath.Join(basePath, "units", g.UnitNameFor())
}

func (g Identifier) UnitNameFor() string {
	return fmt.Sprintf("gear-%s.service", g)
}

func (g Identifier) UnitNameForJob() string {
	return fmt.Sprintf("job-%s.service", g)
}

func (g Identifier) RepositoryPathFor() string {
	return filepath.Join(basePath, "git", string(g))
}

type Fingerprint []byte

func (f Fingerprint) ToShortName() string {
	return strings.Trim(base64.URLEncoding.EncodeToString(f), "=")
}

func (f Fingerprint) PublicKeyPathFor() string {
	return isolateContentPath(filepath.Join(basePath, "keys", "public"), f.ToShortName(), "")
}

func (i Identifier) GitAccessPathFor(f Fingerprint, write bool) string {
	var access string
	if write {
		access = ".write"
	} else {
		access = ".read"
	}
	return isolateContentPath(filepath.Join(basePath, "access", "git"), string(i), f.ToShortName()+access)
}

func (i Identifier) SshAccessPathFor(f Fingerprint) string {
	return isolateContentPath(filepath.Join(basePath, "access", "gears", "ssh"), string(i), f.ToShortName())
}

func isolateContentPath(base, id, suffix string) string {
	var path string
	if suffix == "" {
		path = filepath.Join(base, id[0:2])
		suffix = id
	} else {
		path = filepath.Join(base, id[0:2], id)
	}
	// fail silently, require startup to set paths, let consumers
	// handle directory not found errors
	os.MkdirAll(path, 0770)
	return filepath.Join(path, suffix)
}

var basePath = "/var/lib/gears"

func VerifyDataPaths() error {
	for _, path := range []string{
		basePath,
		filepath.Join(basePath, "units"),
		filepath.Join(basePath, "git"),
		filepath.Join(basePath, "access", "git", "read"),
		filepath.Join(basePath, "access", "git", "write"),
		filepath.Join(basePath, "access", "gears", "ssh"),
		filepath.Join(basePath, "keys", "public"),
	} {
		if err := checkPath(path, os.FileMode(0770), true); err != nil {
			return err
		}
	}
	return nil
}

func checkPath(path string, mode os.FileMode, dir bool) error {
	stat, err := os.Lstat(path)
	if os.IsNotExist(err) && dir {
		err = os.MkdirAll(path, mode)
		stat, _ = os.Lstat(path)
	}
	if err != nil {
		return errors.New("gear: path (" + path + ") is not available: " + err.Error())
	}
	if stat.IsDir() != dir {
		return errors.New("gear: path (" + path + ") must be a directory instead of a file")
	}
	return nil
}
