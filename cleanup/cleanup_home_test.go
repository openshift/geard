/*
   Copyright 2014 Red Hat, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cleanup

import (
	gocheck "launchpad.net/gocheck"
	"os"
	"path/filepath"
	"testing"
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

	os.Setenv("GEARD_BASE_PATH", basePath)
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

	c.Assert(fileExist(homePath), gocheck.Equals, true)
	c.Assert(fileExist(unitFile), gocheck.Equals, true)

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
