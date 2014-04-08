package jobs

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestMarshalKeyData(t *testing.T) {
	const expectedOutput = "{\"Type\":\"authorized_keys\",\"Value\":\"ssh-rsa foobar\"}"

	key, err := NewKeyData("authorized_keys", "ssh-rsa foobar")
	if err != nil {
		t.Fatal("Unable to create key data", err)
	}
	b, err := json.Marshal(key)
	if err != nil {
		t.Fatal("Unable to marshal key data", err)
	}
	if string(b) != expectedOutput {
		t.Fatal("Marshaled JSON was not correct", string(b))
	}
	data := &KeyData{}
	if err := json.Unmarshal(b, data); err != nil {
		t.Fatal("Unable to unmarshal key data", err)
	}
	if string(data.Value) != "\"ssh-rsa foobar\"" {
		t.Fatal("Value was mangled during unmarshalling", string(data.Value))
	}
	s := ""
	if err := json.Unmarshal(data.Value, &s); err != nil {
		t.Fatal("Unable to unmarshal value to string", err)
	}
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(key); err != nil {
		t.Fatal("Unable to json encode key data", err)
	}
	if buf.String() != expectedOutput+"\n" {
		t.Fatal("Encoded JSON was not correct", buf.String())
	}
}

func TestMarshalKeyPermission(t *testing.T) {
	const expectedOutput = "{\"Type\":\"repository\",\"With\":null}"

	key := &KeyPermission{Type: "repository"}
	b, err := json.Marshal(key)
	if err != nil {
		t.Fatal("Unable to marshal key data", err)
	}
	if string(b) != expectedOutput {
		t.Fatal("Marshaled JSON was not correct", string(b))
	}
}
