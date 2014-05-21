package http

import (
	"os"
	"testing"

	. "github.com/openshift/geard/ssh/jobs"
)

func TestMarshalKeyData(t *testing.T) {
	key, _ := NewKeyData("authorized_keys", "ssh-rsa foobar")
	r := &HttpCreateKeysRequest{
		CreateKeysRequest: CreateKeysRequest{
			&ExtendedCreateKeysData{
				Keys:        []KeyData{*key},
				Permissions: []KeyPermission{},
			},
		},
	}
	r.MarshalHttpRequestBody(os.Stdout)
}
