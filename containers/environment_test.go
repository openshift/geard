package containers_test

import (
	"testing"

	. "github.com/openshift/geard/containers"
)

func TestReadEnvironment(t *testing.T) {
	for _, v := range []envTest{
		envTest{"A=B", "A", "B", true, nil},
		envTest{"A=\"B\"", "A", "B", true, nil},
		envTest{"A='B'", "A", "B", true, nil},
		envTest{"  A=B", "A", "B", true, nil},
		envTest{"  A=B  ", "A", "B", true, nil},
		envTest{"A=\"'B'\"", "A", "'B'", true, nil},
		envTest{"A=\"'B  '\"", "A", "'B  '", true, nil},
		envTest{"A=\"B  \"", "A", "B", true, nil},
	} {
		v.assert(t)
	}
}

type envTest struct {
	source      string
	key         string
	value       string
	success     bool
	expectedErr error
}

func (e *envTest) assert(t *testing.T) {
	env := Environment{}
	ok, err := env.FromString(e.source)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if ok != e.success {
		t.Errorf("Expected '%s' to return %b, %v", e.source, e.success, err)
	}
	if e.key != env.Name {
		t.Errorf("Expected key %s to equal %s", env.Name, e.key)
	}
	if e.value != env.Value {
		t.Errorf("Expected value %s to equal %s", env.Value, e.value)
	}
}
