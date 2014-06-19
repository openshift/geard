package sti

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	. "launchpad.net/gocheck"

	"github.com/fsouza/go-dockerclient"
)

// Register gocheck with the 'testing' runner
func Test(t *testing.T) { TestingT(t) }

type IntegrationTestSuite struct {
	dockerClient *docker.Client
}

// Register IntegrationTestSuite with the gocheck suite manager and add support for 'go test' flags,
// viz: go test -integration
var (
	_ = Suite(&IntegrationTestSuite{})

	integration = flag.Bool("integration", false, "Include integration tests")
)

const (
	DockerSocket = "unix:///var/run/docker.sock"
	TestSource   = "git://github.com/pmorie/simple-html"

	FakeBaseImage       = "sti_test/sti-fake"
	FakeUserImage       = "sti_test/sti-fake-user"
	FakeBrokenBaseImage = "sti_test/sti-fake-broken"

	TagCleanBuild           = "test/sti-fake-app"
	TagCleanBuildUser       = "test/sti-fake-app-user"
	TagIncrementalBuild     = "test/sti-incremental-app"
	TagIncrementalBuildUser = "test/sti-incremental-app-user"

	// Need to serve the scripts from localhost so any potential changes to the
	// scripts are made available for integration testing.
	//
	// Port 23456 must match the port used in the fake image Dockerfiles
	FakeScriptsHttpUrl = "http://localhost:23456/sti-fake/.sti/bin"
)

var (
	FakeScriptsFileUrl string
)

// Suite/Test fixtures are provided by gocheck
func (s *IntegrationTestSuite) SetUpSuite(c *C) {
	if !*integration {
		c.Skip("-integration not provided")
	}

	// get the full path to this .go file so we can construct the file url
	// using this file's dirname
	_, filename, _, _ := runtime.Caller(0)
	testImagesDir := path.Join(path.Dir(filename), "test_images")
	FakeScriptsFileUrl = "file://" + path.Join(testImagesDir, "sti-fake", ".sti", "bin")

	s.dockerClient, _ = docker.NewClient(DockerSocket)
	for _, image := range []string{TagCleanBuild, TagCleanBuildUser, TagIncrementalBuild, TagIncrementalBuildUser} {
		s.dockerClient.RemoveImage(image)
	}

	go http.ListenAndServe(":23456", http.FileServer(http.Dir(testImagesDir)))
	fmt.Printf("Waiting for mock HTTP server to start...")
	if err := waitForHttpReady(); err != nil {
		fmt.Printf("[ERROR] Unable to start mock HTTP server: %s\n", err)
	}
	fmt.Println("done")
}

// Wait for the mock HTTP server to become ready to serve the HTTP requests.
//
func waitForHttpReady() error {
	retryCount := 50
	for {
		if resp, err := http.Get("http://localhost:23456/"); err != nil {
			resp.Body.Close()
			if retryCount -= 1; retryCount > 0 {
				time.Sleep(20 * time.Millisecond)
			} else {
				return err
			}
		} else {
			return nil
		}
	}
}

func (s *IntegrationTestSuite) SetUpTest(c *C) {
}

// TestXxxx methods are identified as test cases

// Test a clean build.  The simplest case.
func (s *IntegrationTestSuite) TestCleanBuild(c *C) {
	s.exerciseCleanBuild(c, TagCleanBuild, false, FakeBaseImage, "")
}

func (s *IntegrationTestSuite) TestCleanBuildUser(c *C) {
	s.exerciseCleanBuild(c, TagCleanBuildUser, false, FakeUserImage, "")
}

func (s *IntegrationTestSuite) TestCleanBuildFileScriptsUrl(c *C) {
	s.exerciseCleanBuild(c, TagCleanBuild, false, FakeBaseImage, FakeScriptsFileUrl)
}

func (s *IntegrationTestSuite) TestCleanBuildHttpScriptsUrl(c *C) {
	s.exerciseCleanBuild(c, TagCleanBuild, false, FakeBaseImage, FakeScriptsHttpUrl)
}

// Test that a build request with a callbackUrl will invoke HTTP endpoint
func (s *IntegrationTestSuite) TestCleanBuildCallbackInvoked(c *C) {
	s.exerciseCleanBuild(c, TagCleanBuild, true, FakeBaseImage, "")
}

func (s *IntegrationTestSuite) exerciseCleanBuild(c *C, tag string, verifyCallback bool, imageName string, scriptsUrl string) {
	callbackUrl := ""
	callbackInvoked := false
	callbackHasValidJson := false
	if verifyCallback {
		handler := func(w http.ResponseWriter, r *http.Request) {
			// we got called
			callbackInvoked = true
			// the header is as expected
			contentType := r.Header["Content-Type"][0]
			callbackHasValidJson = contentType == "application/json"
			// the request body is as expected
			if callbackHasValidJson {
				defer r.Body.Close()
				body, _ := ioutil.ReadAll(r.Body)
				type CallbackMessage struct {
					Payload string
					Success bool
				}
				var callbackMessage CallbackMessage
				err := json.Unmarshal(body, &callbackMessage)
				callbackHasValidJson = (err == nil) && (callbackMessage.Success)
			}
		}
		ts := httptest.NewServer(http.HandlerFunc(handler))
		defer ts.Close()
		callbackUrl = ts.URL
	}

	req := &STIRequest{
		DockerSocket: DockerSocket,
		Verbose:      true,
		BaseImage:    imageName,
		Source:       TestSource,
		Tag:          tag,
		Clean:        true,
		Writer:       os.Stdout,
		CallbackUrl:  callbackUrl,
		ScriptsUrl:   scriptsUrl}

	resp, err := Build(req)

	c.Assert(err, IsNil, Commentf("Sti build failed"))
	c.Assert(resp.Success, Equals, true, Commentf("Sti build failed"))
	c.Assert(callbackInvoked, Equals, verifyCallback, Commentf("Sti build did not invoke callback"))
	c.Assert(callbackHasValidJson, Equals, verifyCallback, Commentf("Sti build did not invoke callback with valid json message"))

	s.checkForImage(c, tag)
	containerId := s.createContainer(c, tag)
	defer s.removeContainer(containerId)
	s.checkBasicBuildState(c, containerId, resp.WorkingDir)
}

// Test an incremental build.
func (s *IntegrationTestSuite) TestIncrementalBuildAndRemovePreviousImage(c *C) {
	s.exerciseIncrementalBuild(c, TagIncrementalBuild, true)
}

func (s *IntegrationTestSuite) TestIncrementalBuildAndKeepPreviousImage(c *C) {
	s.exerciseIncrementalBuild(c, TagIncrementalBuild, true)
}

func (s *IntegrationTestSuite) TestIncrementalBuildUser(c *C) {
	s.exerciseIncrementalBuild(c, TagIncrementalBuildUser, true)
}

func (s *IntegrationTestSuite) exerciseIncrementalBuild(c *C, tag string, removePreviousImage bool) {
	req := &STIRequest{
		DockerSocket:        DockerSocket,
		Verbose:             true,
		BaseImage:           FakeBaseImage,
		Source:              TestSource,
		Tag:                 tag,
		Clean:               true,
		RemovePreviousImage: removePreviousImage,
		Writer:              os.Stdout}

	resp, err := Build(req)
	c.Assert(err, IsNil, Commentf("Sti build failed"))
	c.Assert(resp.Success, Equals, true, Commentf("Sti build failed"))

	previousImageId := resp.ImageID

	req.Clean = false

	resp, err = Build(req)
	c.Assert(err, IsNil, Commentf("Sti build failed"))
	c.Assert(resp.Success, Equals, true, Commentf("Sti build failed"))

	s.checkForImage(c, tag)
	containerId := s.createContainer(c, tag)
	defer s.removeContainer(containerId)
	s.checkIncrementalBuildState(c, containerId, resp.WorkingDir)

	_, err = s.dockerClient.InspectImage(previousImageId)
	if removePreviousImage {
		c.Assert(err, NotNil, Commentf("Previous image %s not deleted", previousImageId))
	} else {
		c.Assert(err, IsNil, Commentf("Coudln't find previous image %s", previousImageId))
	}
}

// Support methods
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
	s.dockerClient.RemoveContainer(docker.RemoveContainerOptions{cId, true, true})
}

func (s *IntegrationTestSuite) checkFileExists(c *C, cId string, filePath string) {
	res := FileExistsInContainer(s.dockerClient, cId, filePath)

	c.Assert(res, Equals, true, Commentf("Couldn't find file %s in container %s", filePath, cId))
}

func (s *IntegrationTestSuite) checkBasicBuildState(c *C, cId string, workingDir string) {
	s.checkFileExists(c, cId, "/sti-fake/assemble-invoked")
	s.checkFileExists(c, cId, "/sti-fake/run-invoked")
	s.checkFileExists(c, cId, "/sti-fake/src/index.html")

	_, err := os.Stat(workingDir)
	c.Assert(err, NotNil)                      // workingDir shouldn't exist, so err should be non-nil
	c.Assert(os.IsNotExist(err), Equals, true) // err should be IsNotExist
}

func (s *IntegrationTestSuite) checkIncrementalBuildState(c *C, cId string, workingDir string) {
	s.checkBasicBuildState(c, cId, workingDir)
	s.checkFileExists(c, cId, "/sti-fake/save-artifacts-invoked")
}
