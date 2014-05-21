package jobs_test

import (
	"errors"
	"testing"

	. "github.com/openshift/geard/jobs"
)

func TestInvokesJobExtension(t *testing.T) {
	test := "foo"
	called := false
	AddJobExtension(JobExtensionFunc(func(r interface{}) (Job, error) {
		if _, ok := r.(string); !ok {
			t.Error("Did not receive the correct value for request", r)
		}
		if r.(string) != "foo" {
			t.Error("Did not receive the correct value for request", r)
		}
		called = true
		return nil, errors.New("Returns error")
	}))

	ret, err := JobFor(test)
	if !called {
		t.Error("Expected extension to be called")
	}
	if ret != nil {
		t.Error("Expected return value to be nil", ret)
	}
	if err == nil {
		t.Error("Expected error to be not nil")
	}
}
