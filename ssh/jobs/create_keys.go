package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/ssh"
	"github.com/openshift/geard/utils"
	"log"
)

type CreateKeysRequest struct {
	*ExtendedCreateKeysData
}

type ExtendedCreateKeysData struct {
	Keys        []KeyData
	Permissions []KeyPermission
}

type KeyPermission struct {
	Type string
	With *utils.RawMessage
}

func NewKeyPermission(t string, value interface{}) (*KeyPermission, error) {
	m, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	raw := utils.RawMessage(m)
	return &KeyPermission{t, &raw}, nil
}

type KeyData struct {
	Type  string
	Value utils.RawMessage
}

func NewKeyData(t string, value interface{}) (*KeyData, error) {
	m, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &KeyData{t, utils.RawMessage(m)}, nil
}

func (d *ExtendedCreateKeysData) Check() error {
	for i := range d.Keys {
		if err := d.Keys[i].Check(); err != nil {
			return err
		}
	}
	for i := range d.Permissions {
		if err := d.Permissions[i].Check(); err != nil {
			return err
		}
	}
	if len(d.Keys) == 0 {
		return errors.New("One or more keys must be specified.")
	}
	if len(d.Permissions) == 0 {
		return errors.New("At least one permission must be specfied.")
	}
	return nil
}

func (k *KeyData) Check() error {
	_, ok := ssh.KeyTypeHandlerFor(k.Type)
	if !ok {
		return errors.New(fmt.Sprintf("The key type '%s' is not recognized.", k.Type))
	}
	if len(k.Value) == 0 {
		return errors.New("Value must be specified.")
	}
	return nil
}

func (p *KeyPermission) Check() error {
	_, ok := ssh.PermissionHandlerFor(p.Type)
	if !ok {
		return errors.New(fmt.Sprintf("The permission type '%s' is not recognized.", p.Type))
	}
	return nil
}

func (k *KeyData) Create() (ssh.KeyLocator, error) {
	handler, ok := ssh.KeyTypeHandlerFor(k.Type)
	if !ok {
		return nil, errors.New(fmt.Sprintf("The key type '%s' is not recognized.", k.Type))
	}
	return handler.CreateKey(k.Value)
}

func (k *KeyPermission) Create(locator ssh.KeyLocator) error {
	handler, ok := ssh.PermissionHandlerFor(k.Type)
	if !ok {
		return errors.New(fmt.Sprintf("The permission type '%s' is not recognized.", k.Type))
	}
	return handler.CreatePermission(locator, k.With)
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

func (j *CreateKeysRequest) Execute(resp jobs.JobResponse) {
	failedKeys := []KeyFailure{}
	for i := range j.Keys {
		key := j.Keys[i]

		locator, err := key.Create()
		if err != nil {
			failedKeys = append(failedKeys, KeyFailure{i, &key, err})
			continue
		}

		for k := range j.Permissions {
			if err := j.Permissions[k].Create(locator); err != nil {
				failedKeys = append(failedKeys, KeyFailure{i, &key, err})
				continue
			}
		}
	}

	if len(failedKeys) > 0 {
		data := make([]KeyStructuredFailure, len(failedKeys))
		for i := range failedKeys {
			data[i] = KeyStructuredFailure{failedKeys[i].Index, failedKeys[i].Reason.Error()}
			log.Printf("Failure %d: %+v", failedKeys[i].Index, failedKeys[i].Reason)
		}
		resp.Failure(jobs.StructuredJobError{jobs.SimpleJobError{jobs.JobResponseError, "Not all keys were completed"}, data})
	} else {
		resp.Success(jobs.JobResponseOk)
	}
}
