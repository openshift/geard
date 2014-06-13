package cleanup

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/geard/config"
)

type EnvCleanup_Test struct{}

var envCleanupTest = &EnvCleanup_Test{}

func Test_EnvCleanup(t *testing.T) {
	envPath, _ := ioutil.TempDir("", "env_path")
	homePath, _ := ioutil.TempDir("", "home_path")
	config.SetContainerBasePath(homePath)

	defer os.RemoveAll(envPath)
	defer os.RemoveAll(homePath)

	indexPath := filepath.Join(envPath, "my")
	envFilePath := filepath.Join(envPath, "my", "my-sample-service1")
	envCleanupTest.setupEnv(envFilePath)

	context, info, error := newContext(false, true)
	plugin := &EnvCleanup{envPath: envPath, homePath: homePath}
	plugin.Clean(context)

	if 0 != error.Len() {
		t.Log(info)
		t.Error(error)
	}

	if fileExist(indexPath) {
		t.Errorf("Failed to remove %s", indexPath)
	}

	if fileExist(envFilePath) {
		t.Errorf("Failed to remove %s", envFilePath)
	}

	// reset environment
	servicePath := filepath.Join(homePath, "home",  "my", "my-sample-service1")
	envCleanupTest.setupEnv(envFilePath)
	envCleanupTest.setupHome(servicePath)
	context, info, error = newContext(false, true)

	plugin.Clean(context)
	if 0 != error.Len() {
		t.Log(info)
		t.Error(error)
	}

	if !fileExist(envFilePath) {
		t.Errorf("Should not have removed %s", envFilePath)
	}
}

func (r *EnvCleanup_Test) setupEnv(envFilePath string) {
	os.MkdirAll(filepath.Dir(envFilePath), (os.FileMode)(0755))
	ioutil.WriteFile(envFilePath, ([]byte)("test"), (os.FileMode)(0600))
}

func (r *EnvCleanup_Test) setupHome(envHomePath string) {
	os.MkdirAll(envHomePath, (os.FileMode)(0755))
}
