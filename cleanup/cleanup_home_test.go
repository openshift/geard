package cleanup

import (
	gocheck "launchpad.net/gocheck"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/geard/config"
)

//Hookup gocheck with go test
func Test(t *testing.T) {
	gocheck.TestingT(t)
}

// This is a fixure used by a suite of tests
type CleanupHomeTestSuite struct{}

var _ = gocheck.Suite(&CleanupHomeTestSuite{})

const (
	basePath = "/tmp/test/containers"
)

func (s *CleanupHomeTestSuite) SetUpSuite(c *gocheck.C) {
	path := filepath.Join(basePath, "home", "te")
	os.MkdirAll(path, (os.FileMode)(0775))

	path = filepath.Join(basePath, "units", "te")
	os.MkdirAll(path, (os.FileMode)(0775))

	// Cannot set GEARD_BASE_PATH as config.init() has already been loaded
	config.SetContainerBasePath(basePath)
}

func (s *CleanupHomeTestSuite) SetUpTest(c *gocheck.C) {

}

func (s *CleanupHomeTestSuite) TearDownTest(c *gocheck.C) {
	os.RemoveAll(basePath)
	c.Assert(fileExist(basePath), gocheck.Equals, false, gocheck.Commentf("basePath: %s", basePath))
}

func (s *CleanupHomeTestSuite) Test_HomeCleanup_Clean_0(c *gocheck.C) {
	homePath := filepath.Join(basePath, "home", "te", "test-service")
	os.MkdirAll(homePath, (os.FileMode)(0775))

	unitFile := filepath.Join(basePath, "units", "te", "ctr-test-service.service")
	file, _ := os.Create(unitFile)
	file.Close()

	context, info, error := newContext(false, true)
	plugin := &HomeCleanup{homePath: filepath.Join(basePath, "home")}
	plugin.Clean(context)

	c.Assert(fileExist(homePath), gocheck.Equals, true, gocheck.Commentf("homePath: %s", homePath))
	c.Assert(fileExist(unitFile), gocheck.Equals, true, gocheck.Commentf("unitFile: %s", unitFile))

	if 0 != error.Len() {
		c.Log(info)
		c.Error(error)
	}
}

func (s *CleanupHomeTestSuite) Test_HomeCleanup_Clean_1(c *gocheck.C) {
	homePath := filepath.Join(basePath, "home", "te", "test-service")
	os.MkdirAll(homePath, (os.FileMode)(0775))

	unitFile := filepath.Join(basePath, "units", "te", "ctr-test-service.service")
	c.Assert(fileExist(unitFile), gocheck.Equals, false, gocheck.Commentf("unitFile: %s", unitFile))

	context, info, error := newContext(false, true)
	plugin := &HomeCleanup{homePath: filepath.Join(basePath, "home")}
	plugin.Clean(context)

	c.Assert(fileExist(homePath), gocheck.Equals, false, gocheck.Commentf("homePath: %s", homePath))

	if 0 != error.Len() {
		c.Log(info)
		c.Error(error)
	}
}
