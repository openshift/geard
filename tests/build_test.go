// +build integration
package tests

import (
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/fsouza/go-dockerclient"
	. "launchpad.net/gocheck"

	cjobs "github.com/openshift/geard/containers/jobs"
	chk "launchpad.net/gocheck"
)

var _ = chk.Suite(&BuildIntegrationTestSuite{})

// if set to false, then to run these tests, run "contrib/tests -a -o -build"
var buildTests = flag.Bool("build", true, "Include build integration tests")

/*
 * The REST API needs a unique request id per HTTP request.
 * This is a quickie impl that will be flaky at scale but is good
 * enough for now, probably
 */
type RequestIdGenerator struct {
	startTime int64
}

func NewRequestIdGenerator() *RequestIdGenerator {
	return &RequestIdGenerator{time.Now().UnixNano()}
}

func (r *RequestIdGenerator) requestId() int {
	return int(time.Now().UnixNano() - r.startTime)
}

type BuildIntegrationTestSuite struct {
	dockerClient *docker.Client
	daemonPort   string
	requestIdGen *RequestIdGenerator
	cmd          *exec.Cmd
}

// Register BuildIntegrationTestSuite with the gocheck suite manager
//var _ = Suite(&BuildIntegrationTestSuite{})

func (s *BuildIntegrationTestSuite) requestId() int {
	return s.requestIdGen.requestId()
}

// Suite/Test fixtures are provided by gocheck
func (s *BuildIntegrationTestSuite) SetUpSuite(c *C) {
	if !*buildTests {
		c.Skip("-build not specified")
	}

	s.dockerClient, _ = docker.NewClient("unix:///var/run/docker.sock")
	s.requestIdGen = NewRequestIdGenerator()
	s.daemonPort = os.Getenv("DAEMON_PORT")

	if s.daemonPort == "" {
		s.daemonPort = "43273"
	}
}

func (s *BuildIntegrationTestSuite) SetUpTest(c *C) {
	s.dockerClient.RemoveImage("geard/fake-app")
}

// TestXxxx methods are identified as test cases
func (s *BuildIntegrationTestSuite) TestBuild(c *C) {
	extendedParams := cjobs.BuildImageRequest{
		Tag:       "geard/fake-app",
		Source:    "git://github.com/pmorie/simple-html",
		BaseImage: "sti_test/sti-fake",
		Clean:     true,
		Verbose:   true,
	}

	// initial/clean build.
	s.buildImage(c, extendedParams)
	s.checkForImage(c, extendedParams.Tag)

	containerId := s.createContainer(c, extendedParams.Tag)
	defer s.removeContainer(containerId)
	s.checkBasicBuildState(c, containerId)

	// incremental build
	extendedParams.Clean = false
	s.buildImage(c, extendedParams)

	incrementalContainerId := s.createContainer(c, extendedParams.Tag)
	defer s.removeContainer(incrementalContainerId)
	s.checkBasicBuildState(c, incrementalContainerId)
	s.checkIncrementalBuildState(c, incrementalContainerId)

}

func (s *BuildIntegrationTestSuite) buildImage(c *C, extendedParams cjobs.BuildImageRequest) {

	cmd := exec.Command("/usr/bin/gear", "build", extendedParams.Source, extendedParams.BaseImage, extendedParams.Tag)
	data, err := cmd.CombinedOutput()
	c.Log(string(data))
	c.Assert(err, chk.IsNil)
}

func (s *BuildIntegrationTestSuite) checkForImage(c *C, tag string) {
	_, err := s.dockerClient.InspectImage(tag)
	c.Assert(err, IsNil, Commentf("Couldn't find built image %s", tag))
}

func (s *BuildIntegrationTestSuite) createContainer(c *C, image string) string {
	config := docker.Config{Image: image, AttachStdout: false, AttachStdin: false}
	container, err := s.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	c.Assert(err, IsNil, Commentf("Couldn't create container from image %s", image))

	err = s.dockerClient.StartContainer(container.ID, &docker.HostConfig{})
	c.Assert(err, IsNil, Commentf("Couldn't start container: %s", container.ID))

	exitCode, err := s.dockerClient.WaitContainer(container.ID)
	c.Assert(exitCode, Equals, 0, Commentf("Bad exit code from container: %d", exitCode))

	return container.ID
}

func (s *BuildIntegrationTestSuite) removeContainer(cId string) {
	s.dockerClient.RemoveContainer(docker.RemoveContainerOptions{cId, true, true})
}

func (s *BuildIntegrationTestSuite) checkFileExists(c *C, cId string, filePath string) {
	err := s.dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{ioutil.Discard, cId, filePath})

	c.Assert(err, IsNil, Commentf("Couldn't find file %s in container %s", filePath, cId))
}

func (s *BuildIntegrationTestSuite) checkBasicBuildState(c *C, cId string) {
	s.checkFileExists(c, cId, "/sti-fake/assemble-invoked")
	s.checkFileExists(c, cId, "/sti-fake/run-invoked")
	s.checkFileExists(c, cId, "/sti-fake/src/index.html")
}

func (s *BuildIntegrationTestSuite) checkIncrementalBuildState(c *C, cId string) {
	s.checkBasicBuildState(c, cId)
	s.checkFileExists(c, cId, "/sti-fake/save-artifacts-invoked")
}
