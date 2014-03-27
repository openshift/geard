package jobs

import (
	"code.google.com/p/go.crypto/ssh"
	"crypto/sha256"
	"errors"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/git"
	"github.com/openshift/geard/utils"
	"log"
	"os"
)

type CreateKeysRequest struct {
	*ExtendedCreateKeysData
}

type ExtendedCreateKeysData struct {
	Keys         []KeyData
	Repositories []RepositoryPermission
	Containers   []ContainerPermission
}

type KeyData struct {
	Type  string
	Value string
}

type RepositoryPermission struct {
	Id    containers.Identifier
	Write bool
}

type ContainerPermission struct {
	Id containers.Identifier
}

func (k *KeyData) Check() error {
	switch k.Type {
	case "authorized_keys":
	default:
		return errors.New("Type must be 'authorized_keys'")
	}
	if k.Value == "" {
		return errors.New("Value must be specified.")
	}
	return nil
}

func (p *RepositoryPermission) Check() error {
	_, err := containers.NewIdentifier(string(p.Id))
	return err
}

func (p *ContainerPermission) Check() error {
	_, err := containers.NewIdentifier(string(p.Id))
	return err
}

func (d *ExtendedCreateKeysData) Check() error {
	for i := range d.Keys {
		if err := d.Keys[i].Check(); err != nil {
			return err
		}
	}
	for i := range d.Containers {
		if err := d.Containers[i].Check(); err != nil {
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
	if len(d.Repositories) == 0 && len(d.Containers) == 0 {
		return errors.New("Either repositories or containers must be specified.")
	}
	return nil
}

type KeyFailure struct {
	Index  int
	Key    *KeyData
	Reason error
}

type KeyStructuredFailure struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
}

func KeyFingerprint(key ssh.PublicKey) utils.Fingerprint {
	bytes := sha256.Sum256(key.Marshal())
	return utils.Fingerprint(bytes[:])
}

func (j *CreateKeysRequest) Execute(resp JobResponse) {
	failedKeys := []KeyFailure{}
	for i := range j.Keys {
		key := j.Keys[i]
		pk, _, _, _, ok := ssh.ParseAuthorizedKey([]byte(key.Value))

		if !ok {
			failedKeys = append(failedKeys, KeyFailure{i, &key, errors.New("Unable to parse key")})
			continue
		}

		value := ssh.MarshalAuthorizedKey(pk)
		fingerprint := KeyFingerprint(pk)
		path := fingerprint.PublicKeyPathFor()

		if err := utils.AtomicWriteToContentPath(path, 0664, value); err != nil {
			failedKeys = append(failedKeys, KeyFailure{i, &key, err})
			continue
		}

		for k := range j.Containers {
			p := j.Containers[k]
			if _, err := os.Stat(p.Id.UnitPathFor()); err != nil {
				failedKeys = append(failedKeys, KeyFailure{i, &key, err})
				continue
			}
			if err := os.Symlink(path, p.Id.SshAccessPathFor(fingerprint)); err != nil && !os.IsExist(err) {
				failedKeys = append(failedKeys, KeyFailure{i, &key, err})
				continue
			}
			if _, err := os.Stat(p.Id.AuthKeysPathFor()); err == nil {
				if err := os.Remove(p.Id.AuthKeysPathFor()); err != nil {
					failedKeys = append(failedKeys, KeyFailure{i, &key, err})
					continue
				}
			}
		}
		for k := range j.Repositories {
			p := j.Repositories[k]
			if _, err := os.Stat(p.Id.RepositoryPathFor()); err != nil {
				failedKeys = append(failedKeys, KeyFailure{i, &key, err})
				continue
			}
			accessPath := p.Id.GitAccessPathFor(fingerprint, p.Write)

			if err := os.Symlink(path, accessPath); err != nil && !os.IsExist(err) {
				failedKeys = append(failedKeys, KeyFailure{i, &key, err})
				continue
			}
			negAccessPath := p.Id.GitAccessPathFor(fingerprint, !p.Write)
			if err := os.Remove(negAccessPath); err != nil && !os.IsNotExist(err) {
				failedKeys = append(failedKeys, KeyFailure{i, &key, err})
				continue
			}
			repoId := git.RepoIdentifier(p.Id)
			if _, err := os.Stat(repoId.AuthKeysPathFor()); err == nil {
				if err := os.Remove(repoId.AuthKeysPathFor()); err != nil && !os.IsNotExist(err) {
					failedKeys = append(failedKeys, KeyFailure{i, &key, err})
					continue
				}
			}
		}
	}

	if len(failedKeys) > 0 {
		data := make([]KeyStructuredFailure, len(failedKeys))
		for i := range failedKeys {
			data[i] = KeyStructuredFailure{failedKeys[i].Index, failedKeys[i].Reason.Error()}
			log.Printf("Failure %d: %+v", failedKeys[i].Index, failedKeys[i].Reason)
		}
		resp.Failure(StructuredJobError{SimpleJobError{JobResponseError, "Not all keys were completed"}, data})
	} else {
		resp.Success(JobResponseOk)
	}
}
