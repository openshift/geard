// +build integration

package tests

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/smarterclayton/geard/jobs"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net/http"
	"os"
	"os/exec"
	"time"
)

var buildTests = flag.Bool("build", false, "Include build integration tests")

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
var _ = Suite(&BuildIntegrationTestSuite{})

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
func (s *BuildIntegrationTestSuite) TestCleanBuild(c *C) {
	extendedParams := jobs.ExtendedBuildImageData{
		Tag:       "geard/fake-app",
		Source:    "git://github.com/pmorie/simple-html",
		BaseImage: "pmorie/sti-fake",
		Clean:     true,
		Verbose:   true,
	}

	s.buildImage(c, extendedParams)
	s.checkForImage(c, extendedParams.Tag)

	containerId := s.createContainer(c, extendedParams.Tag)
	defer s.removeContainer(containerId)
	s.checkBasicBuildState(c, containerId)
}

func (s *BuildIntegrationTestSuite) buildImage(c *C, extendedParams jobs.ExtendedBuildImageData) {
	url := fmt.Sprintf("http://localhost:%s/build-image", s.daemonPort)
	b, _ := json.Marshal(extendedParams)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.ParseForm()

	client := &http.Client{}
	resp, err := client.Do(req)
	c.Assert(err, IsNil, Commentf("Failed to start build"))
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	c.Logf("Response Body: %s", body)
	c.Assert(resp.StatusCode, Equals, 202, Commentf("Bad response: %+v Body: %s", resp, body))
}

func (s *BuildIntegrationTestSuite) checkForImage(c *C, tag string) {
	_, err := s.dockerClient.InspectImage(tag)
	c.Assert(err, IsNil, Commentf("Couldn't find built image"))
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
	s.dockerClient.RemoveContainer(docker.RemoveContainerOptions{cId, true})
}

func (s *BuildIntegrationTestSuite) checkFileExists(c *C, cId string, filePath string) {
	err := s.dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{ioutil.Discard, cId, filePath})

	c.Assert(err, IsNil, Commentf("Couldn't find file %s in container %s", filePath, cId))
}

func (s *BuildIntegrationTestSuite) checkBasicBuildState(c *C, cId string) {
	s.checkFileExists(c, cId, "/sti-fake/prepare-invoked")
	s.checkFileExists(c, cId, "/sti-fake/run-invoked")
	s.checkFileExists(c, cId, "/sti-fake/src/index.html")
}

func (s *BuildIntegrationTestSuite) checkIncrementalBuildState(c *C, cId string) {
	s.checkBasicBuildState(c, cId)
	s.checkFileExists(c, cId, "/sti-fake/save-artifacts-invoked")
}

func (s *BuildIntegrationTestSuite) checkExtendedBuildState(c *C, cId string) {
	s.checkFileExists(c, cId, "/sti-fake/prepare-invoked")
	s.checkFileExists(c, cId, "/sti-fake/run-invoked")
}

func (s *BuildIntegrationTestSuite) checkIncrementalExtendedBuildState(c *C, cId string) {
	s.checkExtendedBuildState(c, cId)
	s.checkFileExists(c, cId, "/sti-fake/src/save-artifacts-invoked")
}
