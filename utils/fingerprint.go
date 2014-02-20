package utils

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"github.com/smarterclayton/geard/config"
)

type Fingerprint []byte

func (f Fingerprint) ToShortName() string {
	return strings.Trim(base64.URLEncoding.EncodeToString(f), "=")
}

func (f Fingerprint) PublicKeyPathFor() string {
	return IsolateContentPath(filepath.Join(config.GearBasePath(), "keys", "public"), f.ToShortName(), "")
}
