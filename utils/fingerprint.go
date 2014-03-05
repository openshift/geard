package utils

import (
	"encoding/base64"
	"github.com/smarterclayton/geard/config"
	"path/filepath"
	"strings"
)

type Fingerprint []byte

func (f Fingerprint) ToShortName() string {
	return strings.Trim(base64.URLEncoding.EncodeToString(f), "=")
}

func (f Fingerprint) PublicKeyPathFor() string {
	return IsolateContentPath(filepath.Join(config.ContainerBasePath(), "keys", "public"), f.ToShortName(), "")
}
