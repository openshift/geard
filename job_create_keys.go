package geard

import (
	"code.google.com/p/go.crypto/ssh"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
)

type createKeysJobRequest struct {
	jobRequest
	UserId string
	Output io.Writer
	Data   *extendedCreateKeysData
}

type KeyData struct {
	Type  string
	Value string
}

func (k *KeyData) Check() error {
	switch k.Type {
	case "ssh-rsa", "ssh-dsa", "ssh-ecdsa":
	default:
		return errors.New("Type must be one of 'ssh-rsa', 'ssh-dsa', or 'ssh-ecdsa'")
	}
	if k.Value == "" {
		return errors.New("Value must be specified.")
	}
	return nil
}

type RepositoryPermission struct {
	Id    Identifier
	Write bool
}

func (p *RepositoryPermission) Check() error {
	_, err := NewIdentifier(string(p.Id))
	return err
}

type GearPermission struct {
	Id Identifier
}

func (p *GearPermission) Check() error {
	_, err := NewIdentifier(string(p.Id))
	return err
}

type extendedCreateKeysData struct {
	Keys         []KeyData
	Repositories []RepositoryPermission
	Gears        []GearPermission
}

func (d *extendedCreateKeysData) Check() error {
	for i := range d.Keys {
		if err := d.Keys[i].Check(); err != nil {
			return err
		}
	}
	for i := range d.Gears {
		if err := d.Gears[i].Check(); err != nil {
			return err
		}
	}
	for i := range d.Repositories {
		if err := d.Repositories[i].Check(); err != nil {
			return err
		}
	}
	if len(d.Keys) == 0 {
		return errors.New("One or more keys must be specified.")
	}
	if len(d.Repositories) == 0 && len(d.Gears) == 0 {
		return errors.New("Either repositories or gears must be specified.")
	}
	return nil
}

type keyFailure struct {
	Index  int
	Key    *KeyData
	Reason error
}

func KeyFingerprint(key ssh.PublicKey) Fingerprint {
	bytes := sha256.Sum256(key.Marshal())
	return Fingerprint(bytes[:])
}

func (j *createKeysJobRequest) Execute() {
	fmt.Fprintf(j.Output, "Enabling keys %s ... \n", j.RequestId)

	failedKeys := []keyFailure{}
	for i := range j.Data.Keys {
		key := j.Data.Keys[i]
		pk, _, _, _, ok := ssh.ParseAuthorizedKey([]byte(key.Value))
		if !ok {
			failedKeys = append(failedKeys, keyFailure{i, &key, errors.New("Unable to parse key")})
			continue
		}

		value := ssh.MarshalAuthorizedKey(pk)
		fingerprint := KeyFingerprint(pk)
		path := fingerprint.PublicKeyPathFor()

		if err := AtomicWriteToContentPath(path, 0660, value); err != nil {
			failedKeys = append(failedKeys, keyFailure{i, &key, err})
			continue
		}

		for k := range j.Data.Gears {
			p := j.Data.Gears[k]
			if _, err := os.Stat(p.Id.UnitPathFor()); err != nil {
				failedKeys = append(failedKeys, keyFailure{i, &key, err})
			}
			if err := os.Symlink(path, p.Id.SshAccessPathFor(fingerprint)); err != nil && !os.IsExist(err) {
				failedKeys = append(failedKeys, keyFailure{i, &key, err})
			}
		}
		for k := range j.Data.Repositories {
			p := j.Data.Repositories[k]
			if _, err := os.Stat(p.Id.RepositoryPathFor()); err != nil {
				failedKeys = append(failedKeys, keyFailure{i, &key, err})
			}
			accessPath := p.Id.GitAccessPathFor(fingerprint, p.Write)
			if err := os.Symlink(path, accessPath); err != nil && !os.IsExist(err) {
				failedKeys = append(failedKeys, keyFailure{i, &key, err})
			}
		}
	}
	// FIXME Execute should take an interface for reporting errors, completion, etc
	// that abstracts differences between JSON serialization, message transport
	// serialization, etc
	for i := range failedKeys {
		fmt.Fprintf(j.Output, "Failure %i: %+v", failedKeys[i].Index, failedKeys[i].Reason)
	}
}
