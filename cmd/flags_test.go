package cmd_test

import (
	"testing"

	. "github.com/openshift/geard/cmd"
)

func TestGenerateId(t *testing.T) {
	s := GenerateId()
	if s == "" {
		t.Error("Expected generated ID to be non empty")
	}
}
