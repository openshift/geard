package cmd_test

import (
	"testing"

	. "github.com/openshift/geard/containers/cmd"
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
}

func (t *testTransport) LocatorFor(locator string) (transport.Locator, error) {
	t.GotLocator = locator
	return &testLocator{locator}, nil
}
func (t *testTransport) RemoteJobFor(locator transport.Locator, job interface{}) (jobs.Job, error) {
	panic("should not be called")
}

func TestShouldExtractLocators(t *testing.T) {
	trans := &testTransport{}
	args := []string{}
	err := ExtractContainerLocatorsFromDeployment(trans, "../../deployment/fixtures/mongo_deploy_existing.json", &args)
	if err != nil {
		t.Fatalf("Expected no error from extract: %+v", err)
	}
	if len(args) != 3 {
		t.Fatalf("Expected args to have 3 locators, not %d", len(args))
	}
}
