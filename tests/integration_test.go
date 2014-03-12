package tests

import (
	"fmt"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/docker"
	"github.com/smarterclayton/geard/systemd"
	chk "launchpad.net/gocheck"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const CONTAINER_STABIZE_TIMEOUT = time.Second
const CONTAINER_STATE_CHANGE_TIMEOUT = 15

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

func (s *IntegrationTestSuite) getContainerPid(id containers.Identifier) int {
	container, _ := s.dockerClient.GetContainer(id.ContainerFor(), true)
	return container.State.Pid
}

func (s *IntegrationTestSuite) assertContainerState(c *chk.C, id containers.Identifier, state string) {
	time.Sleep(CONTAINER_STABIZE_TIMEOUT) //wait for state to stabalize
	switch {
	case state == "deleted":
		for i := 0; i < CONTAINER_STATE_CHANGE_TIMEOUT; i++ {
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
		for i := 0; i < CONTAINER_STATE_CHANGE_TIMEOUT; i++ {
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

	containers, err := s.dockerClient.ListContainers()
	c.Assert(err, chk.IsNil)
	for _, cinfo := range containers {
		container, err := s.dockerClient.GetContainer(cinfo.ID, false)
		c.Assert(err, chk.IsNil)
		if strings.HasPrefix(container.Name, "IntTest") {
			s.dockerClient.ForceCleanContainer(cinfo.ID)
		}
	}
}

func (s *IntegrationTestSuite) SetupTest(c *chk.C) {
}

func (s *IntegrationTestSuite) TearDownTest(c *chk.C) {
}

func (s *IntegrationTestSuite) TestRestartContainer(c *chk.C) {
	id, err := containers.NewIdentifier("IntTest004")
	c.Assert(err, chk.IsNil)
	s.containerIds = append(s.containerIds, id)

	hostContainerId := fmt.Sprintf("%v/%v", s.daemonURI, id)

	cmd := exec.Command("/var/lib/containers/bin/gear", "install", "pmorie/sti-html-app", hostContainerId, "--ports=8080:4002", "--start")
	data, err := cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertFilePresent(c, id.UnitPathFor(), 0664, true)
	s.assertContainerState(c, id, "running")
	s.assertFilePresent(c, filepath.Join(id.HomePath(), "container-init.sh"), 0700, false)
	oldPid := s.getContainerPid(id)

	cmd = exec.Command("/var/lib/containers/bin/gear", "restart", hostContainerId)
	data, err = cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)

	sdconn, errc := systemd.NewConnection()
	c.Assert(errc, chk.IsNil)
	err = sdconn.Subscribe()
	c.Assert(errc, chk.IsNil)
	defer sdconn.Unsubscribe()
	sdchan, errchan := sdconn.SubscribeUnits(time.Second)

	var didStop bool
	var didReStart bool
	for true {
		select {
		case unitstatus := <-sdchan:
			status := unitstatus["container-IntTest004.service"]
			c.Log(status)
			if status != nil {
				if status.ActiveState == "deactivating" {
					didStop = true
					c.Logf("%v %v", didStop, didReStart)
				}
				if didStop && status.ActiveState == "active" {
					c.Logf("%v %v", didStop, didReStart)
					didReStart = true
				}
			}
		case err := <-errchan:
			c.Assert(err, chk.IsNil)
		case <-time.After(time.Minute):
			c.Logf("%v %v", didStop, didReStart)
			c.Log("Timed out during restart")
			c.Assert(1, chk.Equals, 2)
		}
		if didReStart {
			break
		}
	}

	newPid := s.getContainerPid(id)
	c.Assert(oldPid, chk.Not(chk.Equals), newPid)
}

func (s *IntegrationTestSuite) TestStartStopContainer(c *chk.C) {
	id, err := containers.NewIdentifier("IntTest003")
	c.Assert(err, chk.IsNil)
	s.containerIds = append(s.containerIds, id)

	hostContainerId := fmt.Sprintf("%v/%v", s.daemonURI, id)

	cmd := exec.Command("/var/lib/containers/bin/gear", "install", "pmorie/sti-html-app", hostContainerId, "--ports=8080:4001")
	data, err := cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertFilePresent(c, id.UnitPathFor(), 0664, true)

	cmd = exec.Command("/var/lib/containers/bin/gear", "start", hostContainerId)
	data, err = cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertContainerState(c, id, "running")
	s.assertFilePresent(c, filepath.Join(id.HomePath(), "container-init.sh"), 0700, false)

	ports, err := containers.GetExistingPorts(id)
	c.Assert(err, chk.IsNil)
	resp, err := http.Get(fmt.Sprintf("http://0.0.0.0:%v", ports[0].External))
	c.Assert(err, chk.IsNil)
	c.Assert(resp.StatusCode, chk.Equals, 200)

	cmd = exec.Command("/var/lib/containers/bin/gear", "stop", hostContainerId)
	data, err = cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertContainerState(c, id, "deleted")
}

func (s *IntegrationTestSuite) TestIsolateInstallAndStartImage(c *chk.C) {
	id, err := containers.NewIdentifier("IntTest001")
	c.Assert(err, chk.IsNil)
	s.containerIds = append(s.containerIds, id)

	hostContainerId := fmt.Sprintf("%v/%v", s.daemonURI, id)

	cmd := exec.Command("/var/lib/containers/bin/gear", "install", "pmorie/sti-html-app", hostContainerId, "--start", "--ports=8080:4000")
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
