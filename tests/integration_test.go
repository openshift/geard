package tests

import (
	"fmt"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/docker"
	chk "launchpad.net/gocheck"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

//Hookup gocheck with go test
func Test(t *testing.T) {
	chk.TestingT(t)
}

var _ = chk.Suite(&IntegrationTestSuite{})

type IntegrationTestSuite struct {
	dockerClient  *docker.DockerClient
	daemonURI     string
	containerIds  []containers.Identifier
	repositoryIds []string
}

func (s *IntegrationTestSuite) assertFilePresent(c *chk.C, path string, perm os.FileMode, readableByNobodyUser bool) {
	info, err := os.Stat(path)
	c.Assert(err, chk.IsNil)
	if (info.Mode() & os.ModeSymlink) != 0 {
		linkedFile, err := os.Readlink(path)
		c.Assert(err, chk.IsNil)
		s.assertFilePresent(c, linkedFile, perm, readableByNobodyUser)
	} else {
		c.Assert(info.Mode().Perm(), chk.Equals, perm)
	}

	if readableByNobodyUser {
		for i := path; i != "/"; i = filepath.Dir(i) {
			info, err = os.Stat(i)
			c.Assert(err, chk.IsNil)
			c.Assert(info.Mode().Perm()&0005, chk.Not(chk.Equals), 0)
		}
	}
}

func (s *IntegrationTestSuite) assertFileAbsent(c *chk.C, path string) {
	c.Logf("assertFileAbsent(%v,%v,%v)", path)
	_, err := os.Stat(path)
	c.Assert(err, chk.Not(chk.IsNil))
}

func (s *IntegrationTestSuite) assertContainerState(c *chk.C, id containers.Identifier, state string) {
	switch {
	case state == "deleted":
		for i := 0; i < 15; i++ {
			_, err := s.dockerClient.GetContainer(id.ContainerFor(), false)
			if err == nil {
				time.Sleep(time.Second)
			} else {
				break
			}
		}
		_, err := s.dockerClient.GetContainer(id.ContainerFor(), false)
		c.Assert(err, chk.Not(chk.IsNil))
	case state == "running":
		for i := 0; i < 10; i++ {
			container, _ := s.dockerClient.GetContainer(id.ContainerFor(), true)
			if !container.State.Running {
				time.Sleep(time.Second)
			} else {
				break
			}
		}
		container, err := s.dockerClient.GetContainer(id.ContainerFor(), true)
		c.Assert(err, chk.IsNil)
		c.Assert(container.State.Running, chk.Equals, true)
	case state == "stopped":
		container, err := s.dockerClient.GetContainer(id.ContainerFor(), true)
		c.Assert(err, chk.IsNil)
		c.Assert(container.State.Running, chk.Equals, false)
	}
}

func (s *IntegrationTestSuite) SetUpSuite(c *chk.C) {
	var err error

	travis := os.Getenv("TRAVIS")
	if travis != "" {
		c.Skip("-skip run on Travis")
	}

	s.daemonURI = os.Getenv("GEARD_URI")
	if s.daemonURI == "" {
		s.daemonURI = "localhost:8080"
	}

	dockerURI := os.Getenv("DOCKER_URI")
	if dockerURI == "" {
		dockerURI = "unix:///var/run/docker.sock"
	}
	s.dockerClient, err = docker.GetConnection(dockerURI)
	c.Assert(err, chk.IsNil)
}

func (s *IntegrationTestSuite) SetupTest(c *chk.C) {

}

func (s *IntegrationTestSuite) TearDownTest(c *chk.C) {
	for _, id := range s.containerIds {
		hostContainerId := fmt.Sprintf("%v/%v", s.daemonURI, id)

		cmd := exec.Command("/var/lib/containers/bin/gear", "delete", hostContainerId)
		data, err := cmd.CombinedOutput()
		c.Log(string(data))
		if err != nil {
			c.Logf("Container %v did not cleanup properly", id)
		}
	}
}

func (s *IntegrationTestSuite) TestIsolateInstallAndStartImage(c *chk.C) {
	id, err := containers.NewIdentifier("IntTest001")
	c.Assert(err, chk.IsNil)
	s.containerIds = append(s.containerIds, id)

	hostContainerId := fmt.Sprintf("%v/%v", s.daemonURI, id)

	cmd := exec.Command("/var/lib/containers/bin/gear", "install", "pmorie/sti-html-app", hostContainerId, "--ports=8080:4000", "--start")
	data, err := cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertContainerState(c, id, "running")

	s.assertFilePresent(c, id.UnitPathFor(), 0664, true)
	paths, err := filepath.Glob(id.VersionedUnitPathFor("*"))
	c.Assert(err, chk.IsNil)
	for _, p := range paths {
		s.assertFilePresent(c, p, 0664, true)
	}
	s.assertFilePresent(c, filepath.Join(id.HomePath(), "container-init.sh"), 0700, false)

	ports, err := containers.GetExistingPorts(id)
	c.Assert(err, chk.IsNil)
	resp, err := http.Get(fmt.Sprintf("http://0.0.0.0:%v", ports[0].External))
	c.Assert(err, chk.IsNil)
	c.Assert(resp.StatusCode, chk.Equals, 200)
}

func (s *IntegrationTestSuite) TestIsolateInstallImage(c *chk.C) {
	id, err := containers.NewIdentifier("IntTest002")
	c.Assert(err, chk.IsNil)
	s.containerIds = append(s.containerIds, id)

	hostContainerId := fmt.Sprintf("%v/%v", s.daemonURI, id)

	cmd := exec.Command("/var/lib/containers/bin/gear", "install", "pmorie/sti-html-app", hostContainerId)
	data, err := cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertContainerState(c, id, "deleted") //never started

	s.assertFilePresent(c, id.UnitPathFor(), 0664, true)
	paths, err := filepath.Glob(id.VersionedUnitPathFor("*"))
	c.Assert(err, chk.IsNil)
	for _, p := range paths {
		s.assertFilePresent(c, p, 0664, true)
	}
}

func (s *IntegrationTestSuite) TearDownSuite(c *chk.C) {
}
