package ssh

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	key "github.com/openshift/geard/pkg/ssh-public-key"
	"github.com/openshift/geard/utils"
)

func init() {
	AddKeyTypeHandler("authorized_keys", &authorizedKeyType{})
}

type authorizedKeyType struct{}

func (t authorizedKeyType) CreateKey(raw utils.RawMessage) (KeyLocator, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, errors.New("The key value must be a string in the authorized_keys format.")
	}

	pk, _, _, _, ok := key.ParseAuthorizedKey([]byte(value))
	if !ok {
		return nil, errors.New("Unable to parse the provided key")
	}

	contents := key.MarshalAuthorizedKey(pk)
	fingerprint := KeyFingerprint(pk)
	path := fingerprint.PublicKeyPathFor()

	if err := utils.AtomicWriteToContentPath(path, 0664, contents); err != nil {
		return nil, err
	}
	return &SimpleKeyLocator{path, fingerprint.ToShortName()}, nil
}

func KeyFingerprint(key key.PublicKey) utils.Fingerprint {
	bytes := sha256.Sum256(key.Marshal())
	return utils.Fingerprint(bytes[:])
}
