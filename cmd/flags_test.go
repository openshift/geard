package cmd_test

import (
	. "github.com/openshift/geard/cmd"
	"testing"
)

func TestGenerateId(t *testing.T) {
	s := GenerateId()
	if s == "" {
		t.Error("Expected generated ID to be non empty")
	}
}
