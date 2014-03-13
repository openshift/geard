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
    "flag"
	"testing"
	"time"
)

const (
	CONTAINER_STATE_CHANGE_TIMEOUT = time.Minute
	DOCKER_STATE_CHANGE_TIMEOUT    = time.Minute
	SYSTEMD_ACTION_DELAY           = time.Second * 2
)

var skipInt = flag.Bool("skip-integration", false, "Skip integration tests")

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
	sdconn        systemd.Systemd
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

const (
	CONTAINER_CREATED ContainerState = iota
	CONTAINER_STARTED
	CONTAINER_RESTARTED
	CONTAINER_STOPPED
)

type ContainerState int

func (c ContainerState) String() string {
	switch {
	case c == CONTAINER_CREATED:
		return "created"
	case c == CONTAINER_STARTED:
		return "started"
	case c == CONTAINER_RESTARTED:
		return "restarted"
	case c == CONTAINER_STOPPED:
		return "stopped"
	default:
		return "unknown"
	}
}

func (s *IntegrationTestSuite) assertContainerState(c *chk.C, id containers.Identifier, expectedState ContainerState) {
	var (
		curState   ContainerState
		didStop    bool
		didRestart bool
		ticker     *time.Ticker
	)

	ticker = time.NewTicker(time.Second)
	defer ticker.Stop()

	cInfo, err := s.sdconn.GetUnitProperties(id.UnitNameFor())
	c.Assert(err, chk.IsNil)
	switch {
	case cInfo["SubState"] == "running":
		curState = CONTAINER_STARTED
	case cInfo["SubState"] == "dead" || cInfo["SubState"] == "failed" || cInfo["SubState"] == "stop-sigterm":
		didStop = true
		curState = CONTAINER_STOPPED
	}
	c.Logf("Current state: %v, interpreted as %v", cInfo["SubState"], curState)

	if curState != expectedState {
		for true {
			select {
			case <-ticker.C:
				cInfo, err := s.sdconn.GetUnitProperties(id.UnitNameFor())
				c.Assert(err, chk.IsNil)
				switch {
				case cInfo["SubState"] == "running":
					curState = CONTAINER_STARTED
					if didStop {
						didRestart = true
					}
				case cInfo["SubState"] == "dead" || cInfo["SubState"] == "failed" || cInfo["SubState"] == "stop-sigterm":
					didStop = true
					curState = CONTAINER_STOPPED
				}
				c.Logf("Current state: %v, interpreted as %v", cInfo["SubState"], curState)
			case <-time.After(CONTAINER_STATE_CHANGE_TIMEOUT):
				c.Logf("%v %v", didStop, didRestart)
				c.Log("Timed out during state change")
				c.Assert(1, chk.Equals, 2)
			}
			if (curState == expectedState) || (expectedState == CONTAINER_RESTARTED && didRestart == true) {
				break
			}
		}
	}

	switch {
	case expectedState == CONTAINER_STOPPED:
		for true {
			select {
			case <-ticker.C:
				_, err := s.dockerClient.GetContainer(id.ContainerFor(), false)
				if err != nil {
					return
				}
			case <-time.After(DOCKER_STATE_CHANGE_TIMEOUT):
				c.Log("Timed out waiting for docker container to stop")
				c.FailNow()
			}
		}
	case expectedState == CONTAINER_STARTED || expectedState == CONTAINER_RESTARTED:
		for true {
			select {
			case <-ticker.C:
				container, err := s.dockerClient.GetContainer(id.ContainerFor(), true)
				if err != nil {
					continue
				}
				c.Logf("Container state: %v. Info: %v", container.State.Running, container.State)
				if container.State.Running {
					return
				}
			case <-time.After(DOCKER_STATE_CHANGE_TIMEOUT):
				c.Log("Timed out waiting for docker container to start")
				c.FailNow()
			}
		}
	}
}

func (s *IntegrationTestSuite) SetUpSuite(c *chk.C) {
	var err error
    
	if *skipInt {
		c.Skip("--build not specified")
	}

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

	s.sdconn, err = systemd.NewConnection()
	c.Assert(err, chk.IsNil)
	err = s.sdconn.Subscribe()
	c.Assert(err, chk.IsNil)
	defer s.sdconn.Unsubscribe()
}

func (s *IntegrationTestSuite) SetupTest(c *chk.C) {
}

func (s *IntegrationTestSuite) TearDownTest(c *chk.C) {
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
	s.assertContainerState(c, id, CONTAINER_STARTED)

	s.assertFilePresent(c, id.UnitPathFor(), 0664, true)
	paths, err := filepath.Glob(id.VersionedUnitPathFor("*"))
	c.Assert(err, chk.IsNil)
	for _, p := range paths {
		s.assertFilePresent(c, p, 0664, true)
	}
	s.assertFilePresent(c, filepath.Join(id.HomePath(), "container-init.sh"), 0700, false)

	ports, err := containers.GetExistingPorts(id)
	c.Assert(err, chk.IsNil)

	t := time.NewTicker(time.Second)
	defer t.Stop()
	select {
	case <-t.C:
		resp, err := http.Get(fmt.Sprintf("http://0.0.0.0:%v", ports[0].External))
		if err == nil {
			c.Assert(resp.StatusCode, chk.Equals, 200)
		}
	case <-time.After(time.Second * 15):
		c.Fail()
	}
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
	s.assertContainerState(c, id, CONTAINER_STOPPED) //never started

	s.assertFilePresent(c, id.UnitPathFor(), 0664, true)
	paths, err := filepath.Glob(id.VersionedUnitPathFor("*"))
	c.Assert(err, chk.IsNil)
	for _, p := range paths {
		s.assertFilePresent(c, p, 0664, true)
	}
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
	s.assertContainerState(c, id, CONTAINER_STARTED)
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
	s.assertContainerState(c, id, CONTAINER_STOPPED)
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
	s.assertContainerState(c, id, CONTAINER_STARTED)
	s.assertFilePresent(c, filepath.Join(id.HomePath(), "container-init.sh"), 0700, false)
	oldPid := s.getContainerPid(id)

	cmd = exec.Command("/var/lib/containers/bin/gear", "restart", hostContainerId)
	data, err = cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertContainerState(c, id, CONTAINER_RESTARTED)

	newPid := s.getContainerPid(id)
	c.Assert(oldPid, chk.Not(chk.Equals), newPid)
}

func (s *IntegrationTestSuite) TestStatus(c *chk.C) {
	id, err := containers.NewIdentifier("IntTest005")
	c.Assert(err, chk.IsNil)
	s.containerIds = append(s.containerIds, id)

	hostContainerId := fmt.Sprintf("%v/%v", s.daemonURI, id)

	cmd := exec.Command("/var/lib/containers/bin/gear", "install", "pmorie/sti-html-app", hostContainerId)
	data, err := cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertFilePresent(c, id.UnitPathFor(), 0664, true)

	cmd = exec.Command("/var/lib/containers/bin/gear", "status", hostContainerId)
	data, err = cmd.CombinedOutput()
	c.Assert(err, chk.IsNil)
	c.Log(string(data))
	c.Assert(strings.Contains(string(data), "Loaded: loaded (/var/lib/containers/units/In/ctr-IntTest005.service; enabled)"), chk.Equals, true)
	s.assertContainerState(c, id, CONTAINER_STOPPED)

	cmd = exec.Command("/var/lib/containers/bin/gear", "start", hostContainerId)
	_, err = cmd.CombinedOutput()
	c.Assert(err, chk.IsNil)
	s.assertContainerState(c, id, CONTAINER_STARTED)

	cmd = exec.Command("/var/lib/containers/bin/gear", "status", hostContainerId)
	data, err = cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	c.Assert(strings.Contains(string(data), "Loaded: loaded (/var/lib/containers/units/In/ctr-IntTest005.service; enabled)"), chk.Equals, true)
	c.Assert(strings.Contains(string(data), "Active: active (running)"), chk.Equals, true)

	cmd = exec.Command("/var/lib/containers/bin/gear", "stop", hostContainerId)
	_, err = cmd.CombinedOutput()
	c.Assert(err, chk.IsNil)
	s.assertContainerState(c, id, CONTAINER_STOPPED)

	cmd = exec.Command("/var/lib/containers/bin/gear", "status", hostContainerId)
	data, err = cmd.CombinedOutput()
	c.Assert(err, chk.IsNil)
	c.Log(string(data))
	c.Assert(strings.Contains(string(data), "Loaded: loaded (/var/lib/containers/units/In/ctr-IntTest005.service; enabled)"), chk.Equals, true)
	c.Assert(strings.Contains(string(data), "Active: inactive (dead)"), chk.Equals, true)
}

func (s *IntegrationTestSuite) TestLongContainerName(c *chk.C) {
	id, err := containers.NewIdentifier("IntTest006xxxxxxxxxxxxxx")
	c.Assert(err, chk.IsNil)
	s.containerIds = append(s.containerIds, id)

	hostContainerId := fmt.Sprintf("%v/%v", s.daemonURI, id)

	cmd := exec.Command("/var/lib/containers/bin/gear", "install", "pmorie/sti-html-app", hostContainerId, "--start", "--ports=8080:4003")
	data, err := cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
	s.assertContainerState(c, id, CONTAINER_STARTED)

	s.assertFilePresent(c, id.UnitPathFor(), 0664, true)
	s.assertFilePresent(c, filepath.Join(id.HomePath(), "container-init.sh"), 0700, false)

	ports, err := containers.GetExistingPorts(id)
	c.Assert(err, chk.IsNil)

	t := time.NewTicker(time.Second)
	defer t.Stop()
	select {
	case <-t.C:
		resp, err := http.Get(fmt.Sprintf("http://0.0.0.0:%v", ports[0].External))
		if err == nil {
			c.Assert(resp.StatusCode, chk.Equals, 200)
		}
	case <-time.After(time.Second * 15):
		c.Fail()
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
