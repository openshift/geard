package geard

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

/*
 * The geard REST api needs a unique request id per HTTP request.
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

// Register gocheck with the 'testing' runner
func Test(t *testing.T) { TestingT(t) }

type IntegrationTestSuite struct {
	dockerClient *docker.Client
	geardPort    string
	requestIdGen *RequestIdGenerator
	geardCmd     *exec.Cmd
}

var integration = flag.Bool("integration", false, "Include integration tests")

// Register IntegrationTestSuite with the gocheck suite manager
var _ = Suite(&IntegrationTestSuite{})

func (s *IntegrationTestSuite) requestId() int {
	return s.requestIdGen.requestId()
}

// Suite/Test fixtures are provided by gocheck
func (s *IntegrationTestSuite) SetUpSuite(c *C) {
	if !*integration {
		c.Skip("-integration not provided")
	}

	travis := os.Getenv("TRAVIS")

	if travis != "" {
		s.geardCmd = s.startGeard(c)
	}

	s.dockerClient, _ = docker.NewClient("unix:///var/run/docker.sock")
	s.requestIdGen = NewRequestIdGenerator()
	s.geardPort = os.Getenv("GEARD_PORT")

	if s.geardPort == "" {
		s.geardPort = "8080"
	}
}

func (s *IntegrationTestSuite) TearDownSuite(c *C) {
	s.stopGeard()
}

func (s *IntegrationTestSuite) startGeard(c *C) *exec.Cmd {
	cmd := exec.Command("sudo", "./geard.local")
	err := cmd.Start()

	c.Assert(err, IsNil, Commentf("Couldn't start geard: %+v", err))

	time.Sleep(30 * time.Second)

	return cmd
}

func (s *IntegrationTestSuite) stopGeard() {
	if s.geardCmd != nil {
		s.geardCmd.Process.Kill()
	}
}

func (s *IntegrationTestSuite) SetUpTest(c *C) {
	s.dockerClient.RemoveImage("geard/fake-app")
}

// TestXxxx methods are identified as test cases
func (s *IntegrationTestSuite) TestCleanBuild(c *C) {
	tag := "geard/fake-app"
	gitRepo := "git://github.com/pmorie/simple-html"
	baseImage := "pmorie/sti-fake"
	extendedParams := extendedBuildImageData{"", true, true}

	s.buildImage(c, gitRepo, baseImage, tag, extendedParams)
	s.checkForImage(c, tag)

	containerId := s.createContainer(c, tag)
	defer s.removeContainer(containerId)
	s.checkBasicBuildState(c, containerId)
}

func (s *IntegrationTestSuite) buildImage(c *C, sourceRepo string, baseImage string, tag string, extendedParams extendedBuildImageData) {
	// Request parameters are the fields of Token (token.go)
	values := url.Values{}
	values.Set("u", tag)
	values.Set("d", "1")
	values.Set("r", sourceRepo)
	values.Set("t", baseImage)
	values.Set("i", strconv.Itoa(s.requestId()))
	params := values.Encode()

	url := fmt.Sprintf("http://localhost:%s/token/__test__/build-image?%s", s.geardPort, params)
	b, _ := json.Marshal(extendedParams)
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(b))
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

func (s *IntegrationTestSuite) checkForImage(c *C, tag string) {
	_, err := s.dockerClient.InspectImage(tag)
	c.Assert(err, IsNil, Commentf("Couldn't find built image"))
}

func (s *IntegrationTestSuite) createContainer(c *C, image string) string {
	config := docker.Config{Image: image, AttachStdout: false, AttachStdin: false}
	container, err := s.dockerClient.CreateContainer(docker.CreateContainerOptions{Name: "", Config: &config})
	c.Assert(err, IsNil, Commentf("Couldn't create container from image %s", image))

	err = s.dockerClient.StartContainer(container.ID, &docker.HostConfig{})
	c.Assert(err, IsNil, Commentf("Couldn't start container: %s", container.ID))

	exitCode, err := s.dockerClient.WaitContainer(container.ID)
	c.Assert(exitCode, Equals, 0, Commentf("Bad exit code from container: %d", exitCode))

	return container.ID
}

func (s *IntegrationTestSuite) removeContainer(cId string) {
	s.dockerClient.RemoveContainer(docker.RemoveContainerOptions{cId, true})
}

func (s *IntegrationTestSuite) checkFileExists(c *C, cId string, filePath string) {
	err := s.dockerClient.CopyFromContainer(docker.CopyFromContainerOptions{ioutil.Discard, cId, filePath})

	c.Assert(err, IsNil, Commentf("Couldn't find file %s in container %s", filePath, cId))
}

func (s *IntegrationTestSuite) checkBasicBuildState(c *C, cId string) {
	s.checkFileExists(c, cId, "/sti-fake/prepare-invoked")
	s.checkFileExists(c, cId, "/sti-fake/run-invoked")
	s.checkFileExists(c, cId, "/sti-fake/src/index.html")
}

func (s *IntegrationTestSuite) checkIncrementalBuildState(c *C, cId string) {
	s.checkBasicBuildState(c, cId)
	s.checkFileExists(c, cId, "/sti-fake/save-artifacts-invoked")
}

func (s *IntegrationTestSuite) checkExtendedBuildState(c *C, cId string) {
	s.checkFileExists(c, cId, "/sti-fake/prepare-invoked")
	s.checkFileExists(c, cId, "/sti-fake/run-invoked")
}

func (s *IntegrationTestSuite) checkIncrementalExtendedBuildState(c *C, cId string) {
	s.checkExtendedBuildState(c, cId)
	s.checkFileExists(c, cId, "/sti-fake/src/save-artifacts-invoked")
}
