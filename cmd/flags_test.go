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

func TestExtractVariablesFrom_NoAppend(t *testing.T) {
	args := []string{"arg1", "a=b", "arg2", "c=d"}

	env := EnvironmentDescription{}

	if err := env.ExtractVariablesFrom(&args, false); err != nil {
		t.Error("Unexpected error parsing arguments")
	}

	if len(env.Description.Variables) != 2 {
		t.Error("Too few parsed arguments")
	}

	if env.Description.Variables[0].Name != "a" &&
		env.Description.Variables[0].Value != "b" &&
		env.Description.Variables[1].Name != "c" &&
		env.Description.Variables[1].Value != "d" {
		t.Error("Incorrect argument parsing")
	}
}
