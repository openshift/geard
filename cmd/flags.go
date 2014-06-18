package cmd

import (
	"crypto/rand"

	"github.com/openshift/geard/utils"
)

func GenerateId() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return utils.Fingerprint(b).ToShortName()
}
