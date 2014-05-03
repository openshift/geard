package cleanup

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	info_payload = `{
		"Containers":1,
		"Debug":0,
		"Driver":"devicemapper",
		"DriverStatus":[
		  [
			 "Pool Name",
			 "docker-253:1-131550-pool"
		  ],
		  [
			 "Data file",
			 "/var/lib/docker/devicemapper/devicemapper/data"
		  ],
		  [
			 "Metadata file",
			 "/var/lib/docker/devicemapper/devicemapper/metadata"
		  ],
		  [
			 "Data Space Used",
			 "851.8 Mb"
		  ],
		  [
			 "Data Space Total",
			 "102400.0 Mb"
		  ],
		  [
			 "Metadata Space Used",
			 "1.5 Mb"
		  ],
		  [
			 "Metadata Space Total",
			 "2048.0 Mb"
		  ]
		],
		"ExecutionDriver":"native-0.1",
		"IPv4Forwarding":1,
		"Images":12,
		"IndexServerAddress":"https://index.docker.io/v1/",
		"InitPath":"/usr/libexec/docker/dockerinit",
		"InitSha1":"3c3b2fcf8aee1e5df319637272c06763e3d81896",
		"KernelVersion":"3.11.10-301.fc20.x86_64",
		"MemoryLimit":1,
		"NEventsListener":0,
		"NFd":11,
		"NGoroutines":11,
		"SwapLimit":1
		}`

	containers_payload = `[{
		"Command":"true /bin/sh -c /usr/bin/run",
		"Created":1398871144,
		"Id":"4d84640d81f1c745bc8fdf0726567c8fe9c72201486169fac77540f258c87aef",
		"Image":"pmorie/sti-html-app:latest",
		"Names":[
		  "ctr-sample-service-data"
		],
		"Ports":[
		  {
			"PublicPort":8080,
			"Type":"tcp"
		  }
		] } ]`

	success_payload = `{
		"ID":"ef3e44768c1a3f1aeff7eaeec1b367cb3a1ff70dd20ed716846aabe85be84cdc",
		"Created":"2014-04-30T15:19:05.445139536Z",
		"Path":"/bin/sh",
		"Args":[
		"-c",
		"/usr/bin/run"
		],
		"Config":{
		"Hostname":"ef3e44768c1a",
		"Domainname":"",
		"User":"",
		"Memory":0,
		"MemorySwap":0,
		"CpuShares":0,
		"AttachStdin":false,
		"AttachStdout":true,
		"AttachStderr":true,
		"PortSpecs":null,
		"ExposedPorts":{
		  "8080/tcp":{

		  }
		},
		"Tty":false,
		"OpenStdin":false,
		"StdinOnce":false,
		"Env":[
		  "HOME=/",
		  "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
		],
		"Cmd":[
		  "/bin/sh",
		  "-c",
		  "/usr/bin/run"
		],
		"Dns":null,
		"Image":"pmorie/sti-html-app",
		"Volumes":{

		},
		"VolumesFrom":"ctr-sample-service-data",
		"WorkingDir":"",
		"Entrypoint":null,
		"NetworkDisabled":false,
		"OnBuild":null
		},
		"State":{
		"Running":true,
		"Pid":0,
		"ExitCode":0,
		"StartedAt":"2014-04-30T15:19:05.546852565Z",
		"FinishedAt":"2014-05-01T00:36:11.985592023Z",
		"Ghost":false
		},
		"Image":"f745cf264f86d6b819721e75126ce16af74c7dbb6a9087386a5377dab31f21c7",
		"NetworkSettings":{
		"IPAddress":"",
		"IPPrefixLen":0,
		"Gateway":"",
		"Bridge":"",
		"PortMapping":null,
		"Ports":null
		},
		"ResolvConfPath":"/etc/resolv.conf",
		"HostnamePath":"/var/lib/docker/containers/ef3e44768c1a3f1aeff7eaeec1b367cb3a1ff70dd20ed716846aabe85be84cdc/hostname",
		"HostsPath":"/var/lib/docker/containers/ef3e44768c1a3f1aeff7eaeec1b367cb3a1ff70dd20ed716846aabe85be84cdc/hosts",
		"Name":"ctr-sample-service",
		"Driver":"devicemapper",
		"ExecDriver":"native-0.1",
		"Volumes":{

		},
		"VolumesRW":{

		},
		"HostConfig":{
		"Binds":null,
		"ContainerIDFile":"",
		"LxcConf":[

		],
		"Privileged":false,
		"PortBindings":{
		  "8080/tcp":null
		},
		"Links":null,
		"PublishAllPorts":false
		} }`
)

type FailureCleanup_Test struct{}

var failureCleanupTest = &FailureCleanup_Test{}

func Test_FailureCleanup_Clean_0(t *testing.T) {
	routes := map[string]string{
		"/info":                                                                             info_payload,
		"/containers/json?all=1":                                                            containers_payload,
		"/containers/4d84640d81f1c745bc8fdf0726567c8fe9c72201486169fac77540f258c87aef/json": success_payload,
	}
	server := httptest.NewServer(http.HandlerFunc(failureCleanupTest.newHandler(t, routes)))
	defer server.Close()

	context, info, error := newContext(false, true)

	os.Setenv("DOCKER_URI", server.URL)
	plugin := &FailureCleanup{dockerSocket: server.URL, retentionAge: "0s"}
	plugin.Clean(context)

	if 0 != error.Len() {
		t.Log(info)
		t.Error(error)
	}

	if strings.Contains(info.String(), "Removing container") {
		t.Errorf("Removed container in error: \n%s\n%s", info, error)
	}
}

func Test_FailureCleanup_Clean_1(t *testing.T) {
	routes := map[string]string{
		"/info":                                                                                    info_payload,
		"/containers/json?all=1":                                                                   containers_payload,
		"/containers/4d84640d81f1c745bc8fdf0726567c8fe9c72201486169fac77540f258c87aef/json":        failureCleanupTest.failedPayload(success_payload),
		"/containers/ef3e44768c1a3f1aeff7eaeec1b367cb3a1ff70dd20ed716846aabe85be84cdc/kill":        "{}",
		"/containers/ef3e44768c1a3f1aeff7eaeec1b367cb3a1ff70dd20ed716846aabe85be84cdc?force=1&v=1": "{}",
	}
	server := httptest.NewServer(http.HandlerFunc(failureCleanupTest.newHandler(t, routes)))
	defer server.Close()

	context, info, error := newContext(false, true)

	os.Setenv("DOCKER_URI", server.URL)
	plugin := &FailureCleanup{dockerSocket: server.URL, retentionAge: "0s"}
	plugin.Clean(context)

	if !strings.Contains(info.String(), "Removing container") {
		t.Errorf("Failed to remove container: \n%s\n%s", info, error)
	}
}

func Test_FailureCleanup_Clean_2(t *testing.T) {
	payload := failureCleanupTest.failedPayload(success_payload)
	payload = strings.Replace(payload,
		"\"FinishedAt\":\"2014-05-01T00:36:11.985592023Z\"",
		fmt.Sprintf("\"FinishedAt\":\"%s\"", time.Now().Format(time.RFC3339Nano)),
		1)

	routes := map[string]string{
		"/info":                                                                             info_payload,
		"/containers/json?all=1":                                                            containers_payload,
		"/containers/4d84640d81f1c745bc8fdf0726567c8fe9c72201486169fac77540f258c87aef/json": payload,
	}
	server := httptest.NewServer(http.HandlerFunc(failureCleanupTest.newHandler(t, routes)))
	defer server.Close()

	context, info, error := newContext(false, true)

	os.Setenv("DOCKER_URI", server.URL)
	plugin := &FailureCleanup{dockerSocket: server.URL, retentionAge: "72h"}
	plugin.Clean(context)

	if strings.Contains(info.String(), "Removing container") {
		t.Errorf("Attempted to remove container too early: \n%s\n%s", info, error)
	}

	if 0 != error.Len() {
		t.Log(info)
		t.Error(error)
	}
}

func Test_FailureCleanup_Clean_3(t *testing.T) {
	context, info, error := newContext(false, true)

	plugin := &FailureCleanup{dockerSocket: "scheme://bad/connection", retentionAge: "72h"}
	plugin.Clean(context)

	if !strings.Contains(error.String(), "Unable connect to docker") {
		t.Errorf("Failed to remove container: \n%s\n%s", info, error)
	}
}

func Test_FailureCleanup_Clean_4(t *testing.T) {
	routes := map[string]string{
		"/info":                  info_payload,
		"/containers/json?all=1": "{}",
	}
	server := httptest.NewServer(http.HandlerFunc(failureCleanupTest.newHandler(t, routes)))
	defer server.Close()

	context, info, error := newContext(false, true)

	plugin := &FailureCleanup{dockerSocket: server.URL, retentionAge: "0s"}
	plugin.Clean(context)

	if !strings.Contains(error.String(), "Unable to find any containers") {
		t.Errorf("Unable to find any containers: \n%s\n%s", info, error)
	}
}

func Test_FailureCleanup_Clean_5(t *testing.T) {
	routes := map[string]string{
		"/info":                                                                                    info_payload,
		"/containers/json?all=1":                                                                   containers_payload,
		"/containers/4d84640d81f1c745bc8fdf0726567c8fe9c72201486169fac77540f258c87aef/json":        failureCleanupTest.failedPayload(success_payload),
		"/containers/ef3e44768c1a3f1aeff7eaeec1b367cb3a1ff70dd20ed716846aabe85be84cdc/kill":        "{}",
		"/containers/ef3e44768c1a3f1aeff7eaeec1b367cb3a1ff70dd20ed716846aabe85be84cdc?force=1&v=1": "{}",
	}
	server := httptest.NewServer(http.HandlerFunc(failureCleanupTest.newHandler(t, routes)))
	defer server.Close()

	context, info, error := newContext(true, false)

	plugin := &FailureCleanup{dockerSocket: server.URL, retentionAge: "0s"}
	plugin.Clean(context)

	if !strings.Contains(info.String(), "Removing container ") {
		t.Errorf("Dry run failed: \n%s\n%s", info, error)
	}
}

func Test_FailureCleanup_Clean_6(t *testing.T) {
	payload := strings.Replace(
		failureCleanupTest.failedPayload(success_payload),
		"\"Name\":\"ctr-sample-service\"",
		"\"Name\":\"ctr-sample-service-data\"",
		1)

	routes := map[string]string{
		"/info":                                                                             info_payload,
		"/containers/json?all=1":                                                            containers_payload,
		"/containers/4d84640d81f1c745bc8fdf0726567c8fe9c72201486169fac77540f258c87aef/json": payload,
	}
	server := httptest.NewServer(http.HandlerFunc(failureCleanupTest.newHandler(t, routes)))
	defer server.Close()

	context, info, error := newContext(false, true)

	os.Setenv("DOCKER_URI", server.URL)
	plugin := &FailureCleanup{dockerSocket: server.URL, retentionAge: "72h"}
	plugin.Clean(context)

	if strings.Contains(info.String(), "Removing container") {
		t.Errorf("Attempted to remove data container: \n%s\n%s", info, error)
	}

	if 0 != error.Len() {
		t.Log(info)
		t.Error(error)
	}
}

func (r *FailureCleanup_Test) newHandler(t *testing.T, routes map[string]string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.String()

		for k, v := range routes {
			if k == url {
				fmt.Fprintln(w, v)
				return
			}
		}
		t.Errorf("Unexpected URL: %s", r.URL)
		fmt.Fprintln(w, "{}")
	}
}

func (r *FailureCleanup_Test) failedPayload(payload string) string {
	p := strings.Replace(payload, "\"ExitCode\":0", "\"ExitCode\":100", 1)
	return strings.Replace(p, "\"Running\":true", "\"Running\":false", 1)
}
