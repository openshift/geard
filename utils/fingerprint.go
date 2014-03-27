package utils

import (
	"encoding/base64"
	"github.com/openshift/geard/config"
	"path/filepath"
	"strings"
)

type Fingerprint []byte

func (f Fingerprint) ToShortName() string {
	return strings.Trim(base64.URLEncoding.EncodeToString(f), "=")
}

func (f Fingerprint) PublicKeyPathFor() string {
	return IsolateContentPathWithPerm(filepath.Join(config.ContainerBasePath(), "keys", "public"), f.ToShortName(), "", 0775)
}
