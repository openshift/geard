package locator_test

import (
	"testing"

	"github.com/openshift/geard/cmd"
	. "github.com/openshift/geard/containers/locator"
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
	panic("should not be called")
}

func TestShouldCheckContainerArgs(t *testing.T) {
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

func TestShouldReadIdentifiersFromArgs(t *testing.T) {
	ids, err := cmd.NewResourceLocators(&testTransport{}, ResourceTypeContainer, "ctr://localhost/foo", "bart", "ctr://local/bazi")
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

func TestShouldConvertLocator(t *testing.T) {
	locator := &cmd.ResourceLocator{ResourceTypeContainer, "foobar", &testLocator{"localhost"}}
	id := AsIdentifier(locator)
	if id == "" {
		t.Errorf("Locator should not have error on converting to identifier")
	}
}
