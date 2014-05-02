package cmd_test

import (
	"fmt"
	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/transport"
	"testing"
)

type testLocator struct {
	Locator string
}

func (t *testLocator) String() string {
	return t.Locator
}
func (t *testLocator) ResolveHostname() (string, error) {
	return t.Locator, nil
}

type testTransport struct {
	GotLocator string
	Translated map[string]jobs.Job
	Invoked    map[string]jobs.JobResponse
}

func (t *testTransport) LocatorFor(locator string) (transport.Locator, error) {
	t.GotLocator = locator
	return &testLocator{locator}, nil
}
func (t *testTransport) RemoteJobFor(locator transport.Locator, job jobs.Job) (jobs.Job, error) {
	if t.Translated == nil {
		t.Translated = make(map[string]jobs.Job)
		t.Invoked = make(map[string]jobs.JobResponse)
	}
	t.Translated[locator.String()] = job
	invoked := func(res jobs.JobResponse) {
		if _, found := t.Invoked[locator.String()]; found {
			panic(fmt.Sprintf("Same job %+v invoked twice under %s", job, locator.String()))
		}
		t.Invoked[locator.String()] = res
		res.Success(jobs.JobResponseOk)
	}
	return jobs.JobFunction(invoked), nil
}

func TestShouldSendRemoteJob(t *testing.T) {
	trans := &testTransport{}
	localhost := &testLocator{"localhost"}
	initCalled := false
	locator := &ResourceLocator{ResourceTypeContainer, "foobar", localhost}

	Executor{
		On: Locators{locator},
		Serial: func(on Locator) jobs.Job {
			if on != locator {
				t.Fatalf("Expected locator passed to Serial() to be identical to %+v", locator)
			}
			return &jobs.StoppedContainerStateRequest{
				Id: AsIdentifier(on),
			}
		},
		LocalInit: func() error {
			initCalled = true
			return nil
		},
		Transport: trans,
	}.Gather()

	if initCalled {
		t.Errorf("Local initialization should be bypassed for remote transports")
	}
	if _, ok := trans.Translated["localhost"]; !ok {
		t.Errorf("Job for localhost was not enqueued in %+v", trans.Invoked)
	}
	if _, ok := trans.Invoked["localhost"]; !ok {
		t.Errorf("Job for localhost was not enqueued in %+v", trans.Invoked)
	}
}

func TestShouldExtractLocators(t *testing.T) {
	trans := &testTransport{}
	args := []string{}
	err := ExtractContainerLocatorsFromDeployment(trans, "../deployment/fixtures/mongo_deploy_existing.json", &args)
	if err != nil {
		t.Fatalf("Expected no error from extract: %+v", err)
	}
	if len(args) != 3 {
		t.Fatalf("Expected args to have 3 locators, not %d", len(args))
	}
}

func TestShouldConvertLocator(t *testing.T) {
	locator := &ResourceLocator{ResourceTypeContainer, "foobar", &testLocator{"localhost"}}
	id := AsIdentifier(locator)
	if id == "" {
		t.Errorf("Locator should not have error on converting to identifier")
	}
}

func TestShouldReadIdentifiersFromArgs(t *testing.T) {
	ids, err := NewResourceLocators(&testTransport{}, ResourceTypeContainer, "ctr://localhost/foo", "bart", "ctr://local/bazi")
	if err != nil {
		t.Errorf("No error should occur reading locators: %s", err.Error())
	}
	if len(ids) != 3 {
		t.Errorf("Should have received 3 ids: %+v", ids)
	}
	if string(AsIdentifier(ids[0])) != "" {
		t.Error("First id should have value '' because foo is too short", ids[0])
	}
	if string(AsIdentifier(ids[1])) != "bart" {
		t.Error("Second id should have value 'bart'", ids[1])
	}
	if string(AsIdentifier(ids[2])) != "bazi" {
		t.Error("Third id should have value 'bazi'", ids[2])
	}
}

func TestShoulCheckContainerArgsArgs(t *testing.T) {
	ids, err := NewContainerLocators(&testTransport{}, "ctr://localhost/foo")
	if err == nil {
		t.Errorf("This locator should be invalid: %s", ids[0])
	}
	ids, err = NewContainerLocators(&testTransport{}, "bar")
	if err == nil {
		t.Errorf("This locator should be invalid: %s", ids[0])
	}
	ids, err = NewContainerLocators(&testTransport{}, "ctr://local/baz")
	if err == nil {
		t.Errorf("This locator should be invalid: %s", ids[0])
	}

}
