package cmd_test

import (
	"fmt"
	"testing"

	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/transport"
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
	Translated map[string]interface{}
	Invoked    map[string]jobs.Response
}

func (t *testTransport) LocatorFor(locator string) (transport.Locator, error) {
	t.GotLocator = locator
	return &testLocator{locator}, nil
}
func (t *testTransport) RemoteJobFor(locator transport.Locator, job interface{}) (jobs.Job, error) {
	if t.Translated == nil {
		t.Translated = make(map[string]interface{})
		t.Invoked = make(map[string]jobs.Response)
	}
	t.Translated[locator.String()] = job
	invoked := func(res jobs.Response) {
		if _, found := t.Invoked[locator.String()]; found {
			panic(fmt.Sprintf("Same job %+v invoked twice under %s", job, locator.String()))
		}
		t.Invoked[locator.String()] = res
		res.Success(jobs.ResponseOk)
	}
	return jobs.JobFunction(invoked), nil
}

func TestShouldSendRemoteJob(t *testing.T) {
	trans := &testTransport{}
	localhost := &testLocator{"localhost"}
	initCalled := false
	locator := &ResourceLocator{"ctr", "foobar", localhost}

	Executor{
		On: Locators{locator},
		Serial: func(on Locator) JobRequest {
			if on != locator {
				t.Fatalf("Expected locator passed to Serial() to be identical to %+v", locator)
			}
			id, _ := containers.NewIdentifier(on.(*ResourceLocator).Id)
			return &cjobs.StoppedContainerStateRequest{
				Id: id,
			}
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
