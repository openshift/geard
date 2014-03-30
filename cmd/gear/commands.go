package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	nethttp "net/http"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/openshift/cobra"
	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/deployment"
	"github.com/openshift/geard/dispatcher"
	"github.com/openshift/geard/encrypted"
	"github.com/openshift/geard/git"
	gitcmd "github.com/openshift/geard/git/cmd"
	githttp "github.com/openshift/geard/git/http"
	gitjobs "github.com/openshift/geard/git/jobs"
	"github.com/openshift/geard/http"
	idlercmd "github.com/openshift/geard/idler/cmd"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/go-sti"
)

var (
	listenAddr string

	pre    bool
	post   bool
	follow bool

	resetEnv bool

	start   bool
	isolate bool
	fork    bool
	sockAct bool

	keyPath   string
	expiresAt int64

	environment  EnvironmentDescription
	portPairs    PortPairs
	networkLinks = NetworkLinks{&containers.NetworkLinks{}}

	gitKeys     bool
	gitRepoName string
	gitRepoURL  string

	deploymentPath string

	buildReq    sti.BuildRequest
	keyFile     string
	writeAccess bool
	hostIp      string
)

var conf = http.HttpConfiguration{
	Dispatcher: &dispatcher.Dispatcher{
		QueueFast:         10,
		QueueSlow:         1,
		Concurrent:        2,
		TrackDuplicateIds: 1000,
	},
	Extensions: []http.HttpExtension{
		githttp.Routes,
	},
}

// Parse the command line arguments and invoke one of the support subcommands.
func Execute() {
	gearCmd := &cobra.Command{
		Use:   "gear",
		Short: "Gear(d) is a tool for installing Docker containers to systemd",
		Long:  "A commandline client and server that allows Docker containers to be installed to systemd in an opinionated and distributed fashion.\n\nComplete documentation is available at http://github.com/openshift/geard",
		Run:   gear,
	}
	gearCmd.PersistentFlags().StringVar(&(keyPath), "key-path", "", "Specify the directory containing the server private key and trusted client public keys")
	gearCmd.PersistentFlags().StringVarP(&(conf.Docker.Socket), "docker-socket", "S", "unix:///var/run/docker.sock", "Set the docker socket to use")
	gearCmd.PersistentFlags().StringVarP(&hostIp, "host-ip", "H", GuessHostIp(), "IP address to listen for traffic")
	gearCmd.PersistentFlags().StringVar(&deploymentPath, "with", "", "Provide a deployment descriptor to operate on")

	deployCmd := &cobra.Command{
		Use:   "deploy <file> <host>...",
		Short: "Deploy a set of containers to the named hosts",
		Long:  "Given a simple description of a group of containers, wire them together using the gear primitives.",
		Run:   deployContainers,
	}
	deployCmd.Flags().BoolVar(&isolate, "isolate", false, "Use an isolated container running as a user")
	deployCmd.Flags().BoolVar(&fork, "fork", false, "Use a forked container (experimental, requires docker branch smarterclayton/fork_and_create_only)")
	gearCmd.AddCommand(deployCmd)

	installImageCmd := &cobra.Command{
		Use:   "install <image> <name>... [<env>]",
		Short: "Install a docker image as a systemd service",
		Long:  "Install a docker image as one or more systemd services on one or more servers.\n\nSpecify a location on a remote server with <host>[:<port>]/<name> instead of <name>.  The default port is 2223.",
		Run:   installImage,
	}
	installImageCmd.Flags().VarP(&portPairs, "ports", "p", "List of comma separated port pairs to bind '<internal>:<external>,...'. Use zero to request a port be assigned.")
	installImageCmd.Flags().VarP(&networkLinks, "net-links", "n", "List of comma separated port pairs to wire '<local_host>:<local_port>:<remote_host>:<remote_port>,...'. local_host may be empty. It defaults to 127.0.0.1.")
	installImageCmd.Flags().BoolVar(&start, "start", false, "Start the container immediately")
	installImageCmd.Flags().BoolVar(&isolate, "isolate", false, "Use an isolated container running as a user")
	installImageCmd.Flags().BoolVar(&sockAct, "socket-activated", false, "Use a socket-activated container (experimental, requires Docker branch)")
	installImageCmd.Flags().BoolVar(&fork, "fork", false, "Use a forked container (experimental, requires docker branch openshift/fork_and_create_only)")
	installImageCmd.Flags().StringVar(&environment.Path, "env-file", "", "Path to an environment file to load")
	installImageCmd.Flags().StringVar(&environment.Description.Source, "env-url", "", "A url to download environment files from")
	installImageCmd.Flags().StringVar((*string)(&environment.Description.Id), "env-id", "", "An optional identifier for the environment being set")
	gearCmd.AddCommand(installImageCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <name>...",
		Short: "Delete an installed container",
		Long:  "Deletes one or more installed containers from the system.  Will not clean up unused images.",
		Run:   deleteContainer,
	}
	gearCmd.AddCommand(deleteCmd)

	buildCmd := &cobra.Command{
		Use:   "build <source> <image> <tag> [<env>]",
		Short: "(Local) Build a new image on this host",
		Long:  "Build a new Docker image named <tag> from a source repository and base image.",
		Run:   buildImage,
	}
	buildCmd.Flags().BoolVar(&(buildReq.Clean), "clean", false, "Perform a clean build")
	buildCmd.Flags().StringVar(&(buildReq.WorkingDir), "dir", "tempdir", "Directory where generated Dockerfiles and other support scripts are created")
	buildCmd.Flags().StringVarP(&(buildReq.RuntimeImage), "runtime", "R", "", "Set the runtime image to use")
	buildCmd.Flags().StringVarP(&(buildReq.Method), "method", "m", "run", "Specify a method to build with. build -> 'docker build', run -> 'docker run'")
	buildCmd.Flags().BoolVar(&(buildReq.Debug), "debug", false, "Enable debugging output")
	buildCmd.Flags().StringVar(&environment.Path, "env-file", "", "Path to an environment file to load")
	buildCmd.Flags().StringVar(&environment.Description.Source, "env-url", "", "A url to download environment files from")
	gearCmd.AddCommand(buildCmd)

	setEnvCmd := &cobra.Command{
		Use:   "set-env <name>... [<env>]",
		Short: "Set environment variable values on servers",
		Long:  "Adds the listed environment values to the specified locations. The name is the environment id that multiple containers may reference. You can pass an environment file or key value pairs on the commandline.",
		Run:   setEnvironment,
	}
	setEnvCmd.Flags().BoolVar(&resetEnv, "reset", false, "Remove any existing values")
	setEnvCmd.Flags().StringVar(&environment.Path, "env-file", "", "Path to an environment file to load")
	gearCmd.AddCommand(setEnvCmd)

	envCmd := &cobra.Command{
		Use:   "env <name>...",
		Short: "Retrieve environment variable values by id",
		Long:  "Return the environment variables matching the provided ids",
		Run:   showEnvironment,
	}
	gearCmd.AddCommand(envCmd)

	linkCmd := &cobra.Command{
		Use:   "link <name>...",
		Short: "Set network links for the named containers",
		Long:  "Sets the network links for the named containers. A restart may be required to use the latest links.",
		Run:   linkContainers,
	}
	linkCmd.Flags().VarP(&networkLinks, "net-links", "n", "List of comma separated port pairs to wire '<local_host>:local_port>:<host>:<remote_port>,...'. local_host may be empty. It defaults to 127.0.0.1")
	gearCmd.AddCommand(linkCmd)

	startCmd := &cobra.Command{
		Use:   "start <name>...",
		Short: "Invoke systemd to start a container",
		Long:  "Queues the start and immediately returns.", //  Use -f to attach to the logs.",
		Run:   startContainer,
	}
	//startCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Attach to the logs after startup")
	gearCmd.AddCommand(startCmd)

	stopCmd := &cobra.Command{
		Use:   "stop <name>...",
		Short: "Invoke systemd to stop a container",
		Long:  ``,
		Run:   stopContainer,
	}
	gearCmd.AddCommand(stopCmd)

	restartCmd := &cobra.Command{
		Use:   "restart <name>...",
		Short: "Invoke systemd to restart a container",
		Long:  "Queues the restart and immediately returns.", //  Use -f to attach to the logs.",
		Run:   restartContainer,
	}
	//startCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Attach to the logs after startup")
	gearCmd.AddCommand(restartCmd)

	statusCmd := &cobra.Command{
		Use:   "status <name>...",
		Short: "Retrieve the systemd status of one or more containers",
		Long:  "Shows the equivalent of 'systemctl status ctr-<name>' for each listed unit",
		Run:   containerStatus,
	}
	gearCmd.AddCommand(statusCmd)

	listUnitsCmd := &cobra.Command{
		Use:   "list-units <host>...",
		Short: "Retrieve the list of services across all hosts",
		Long:  "Shows the equivalent of 'systemctl list-units ctr-<name>' for each installed container",
		Run:   listUnits,
	}
	gearCmd.AddCommand(listUnitsCmd)

	sshKeysCmd := &cobra.Command{
		Use:   "add-keys <name_or_repo>...",
		Short: "Set public keys for SSH access to a container",
		Long:  "Upload the provided public keys and enable SSH access to the specified repositories and/or containers.",
		Run:   sshKeysAdd,
	}
	sshKeysCmd.Flags().BoolVar(&writeAccess, "write", false, "Enable write access for the selected keys and repositories")
	sshKeysCmd.Flags().StringVar(&keyFile, "key-file", "", "read input from file specified matching sshd AuthorizedKeysFile format")
	gearCmd.AddCommand(sshKeysCmd)

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "(Local) Start the gear server",
		Long:  "Launch the gear HTTP API server as a daemon. Will not send itself to the background.",
		Run:   daemon,
	}
	daemonCmd.Flags().StringVarP(&listenAddr, "listen-address", "A", ":43273", "Set the address for the http endpoint to listen on")
	gearCmd.AddCommand(daemonCmd)

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "(Local) Disable all containers, slices, and targets in systemd",
		Long:  "Disable all registered resources from systemd to allow them to be removed from the system.  Will reload the systemd daemon config.",
		Run:   clean,
	}
	gearCmd.AddCommand(cleanCmd)

	initGearCmd := &cobra.Command{
		Use:   "init <name> <image>",
		Short: "(Local) Setup the environment for a container",
		Long:  "",
		Run:   initGear,
	}
	initGearCmd.Flags().BoolVarP(&pre, "pre", "", false, "Perform pre-start initialization")
	initGearCmd.Flags().BoolVarP(&post, "post", "", false, "Perform post-start initialization")
	gearCmd.AddCommand(initGearCmd)

	initRepoCmd := &cobra.Command{
		Use:   "init-repo",
		Short: `(Local) Setup the environment for a git repository`,
		Long:  ``,
		Run:   initRepository,
	}
	gearCmd.AddCommand(initRepoCmd)

	genAuthKeysCmd := &cobra.Command{
		Use:   "gen-auth-keys [<name>]",
		Short: "(Local) Create the authorized_keys file for a container or repository",
		Long:  "Generate .ssh/authorized_keys file for the specified container id or (if container id is ommitted) for the current user",
		Run:   genAuthKeys,
	}
	genAuthKeysCmd.Flags().BoolVar(&gitKeys, "git", false, "Create keys for a git repository")
	gearCmd.AddCommand(genAuthKeysCmd)

	sshAuthKeysCmd := &cobra.Command{
		Use:   "auth-keys-command <user name>",
		Short: "(Local) Generate authorized_keys output for sshd.",
		Long:  "Generate authorized keys output for sshd. See sshd_config(5)#AuthorizedKeysCommand",
		Run:   SshAuthKeysCommand,
	}
	gearCmd.AddCommand(sshAuthKeysCmd)

	createTokenCmd := &cobra.Command{
		Use:   "create-token <type> <content_id>",
		Short: "(Local) Generate a content request token",
		Long:  "Create a URL that will serve as a content request token using a server public key and client private key.",
		Run:   createToken,
	}
	createTokenCmd.Flags().Int64Var(&expiresAt, "expires-at", time.Now().Unix()+3600, "Specify the content request token expiration time in seconds after the Unix epoch")
	gearCmd.AddCommand(createTokenCmd)

	idlercmd.LoadCommand(gearCmd, &conf.Docker.Socket, &hostIp)
	gitcmd.LoadCommand(gearCmd)

	if err := gearCmd.Execute(); err != nil {
		Fail(1, err.Error())
	}
}

// Initializers for local command execution.
func needsSystemd() error {
	systemd.Require()
	return nil
}

func needsSystemdAndData() error {
	systemd.Require()
	git.InitializeData()
	return containers.InitializeData()
}

func needsData() error {
	return containers.InitializeData()
}

func gear(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func daemon(cmd *cobra.Command, args []string) {
	api := conf.Handler()
	nethttp.Handle("/", api)

	if keyPath != "" {
		config, err := encrypted.NewTokenConfiguration(filepath.Join(keyPath, "server"), filepath.Join(keyPath, "client.pub"))
		if err != nil {
			Fail(1, "Unable to load token configuration: %s", err.Error())
		}
		nethttp.Handle("/token/", nethttp.StripPrefix("/token", config.Handler(api)))
	}

	systemd.Start()
	containers.InitializeData()
	containers.StartPortAllocator(4000, 60000)
	git.InitializeData()
	conf.Dispatcher.Start()

	log.Printf("Listening for HTTP on %s ...", listenAddr)
	log.Fatal(nethttp.ListenAndServe(listenAddr, nil))
}

func clean(cmd *cobra.Command, args []string) {
	needsSystemd()
	containers.Clean()
}

func deployContainers(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <deployment_file> <host> ...\n")
	}

	path := args[0]
	if path == "" {
		Fail(1, "Argument 1 must be deployment file describing how the containers are related")
	}
	deploy, err := deployment.NewDeploymentFromFile(path)
	if err != nil {
		Fail(1, "Unable to load deployment file: %s", err.Error())
	}
	if len(args) == 1 {
		args = append(args, LocalHostName)
	}
	servers, err := NewHostLocators(args[1:]...)
	if err != nil {
		Fail(1, "You must pass zero or more valid host names (use '%s' or pass no arguments for the current server): %s\n", LocalHostName, err.Error())
	}

	re := regexp.MustCompile("\\.\\d{8}\\-\\d{6}\\z")
	now := time.Now().Format(".20060102-150405")
	base := filepath.Base(path)
	base = re.ReplaceAllString(base, "")
	newPath := base + now

	log.Printf("Deploying %s", newPath)
	changes, removed, err := deploy.Describe(deployment.SimplePlacement(servers))
	if err != nil {
		Fail(1, "Deployment is not valid: %s", err.Error())
	}

	if len(removed) > 0 {
		failures := Executor{
			On: removed.Ids(),
			Serial: func(on Locator) jobs.Job {
				return &http.HttpDeleteContainerRequest{
					Label: on.Identity(),
					DeleteContainerRequest: jobs.DeleteContainerRequest{
						Id: on.(ResourceLocator).Identifier(),
					},
				}
			},
			Output: os.Stdout,
			OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
				fmt.Fprintf(w, "Deleted %s", job.(*http.HttpDeleteContainerRequest).Label)
			},
			LocalInit: needsData,
		}.Stream()
		for i := range failures {
			fmt.Fprintf(os.Stderr, failures[i].Error())
		}
	}

	addedIds := changes.Instances.AddedIds()

	errors := Executor{
		On: addedIds,
		Serial: func(on Locator) jobs.Job {
			instance, _ := changes.Instances.Find(on.(ResourceLocator).Identifier())
			return &http.HttpInstallContainerRequest{
				InstallContainerRequest: jobs.InstallContainerRequest{
					RequestIdentifier: jobs.NewRequestIdentifier(),

					Id:      instance.Id,
					Image:   instance.Image,
					Isolate: isolate,
					Fork:    fork,

					Ports:        instance.Ports.PortPairs(),
					NetworkLinks: instance.Links.NetworkLinks(),
				},
			}
		},
		OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
			instance, _ := changes.Instances.Find(job.(*http.HttpInstallContainerRequest).InstallContainerRequest.Id)
			if pairs, ok := r.Pending["Ports"].(containers.PortPairs); ok {
				if !instance.Ports.Update(pairs) {
					fmt.Fprintf(os.Stderr, "Not all ports listed %+v were returned by the server %+v", instance.Ports, pairs)
				}
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemdAndData,
	}.Stream()

	changes.UpdateLinks()

	contents, _ := json.Marshal(changes)
	if err := ioutil.WriteFile(newPath, contents, 0664); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to write %s: %s\n", newPath, err.Error())
	}

	Executor{
		On: changes.Instances.LinkedIds(),
		Group: func(on ...Locator) jobs.Job {
			links := []jobs.ContainerLink{}
			for i := range on {
				instance, _ := changes.Instances.Find(on[i].(ResourceLocator).Identifier())
				network := instance.Links.NetworkLinks()
				if len(network) > 0 {
					links = append(links, jobs.ContainerLink{instance.Id, network})
				}
			}
			return &http.HttpLinkContainersRequest{
				Label: on[0].HostIdentity(),
				LinkContainersRequest: jobs.LinkContainersRequest{&jobs.ContainerLinks{links}},
			}
		},
		Output: os.Stdout,
		OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
			fmt.Fprintf(w, "Links set on %s\n", job.(*http.HttpLinkContainersRequest).Label)
		},
	}.Stream()

	Executor{
		On: addedIds,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpStartContainerRequest{
				StartedContainerStateRequest: jobs.StartedContainerStateRequest{
					Id: on.(ResourceLocator).Identifier(),
				},
			}
		},
		Output: os.Stdout,
	}.Stream()

	if len(errors) > 0 {
		for i := range errors {
			fmt.Fprintf(os.Stderr, "Error: %s\n", errors[i])
		}
		os.Exit(1)
	}
}

func installImage(cmd *cobra.Command, args []string) {
	if err := environment.ExtractVariablesFrom(&args, true); err != nil {
		Fail(1, err.Error())
	}

	if len(args) < 2 {
		Fail(1, "Valid arguments: <image_name> <id> ...\n")
	}

	imageId := args[0]
	if imageId == "" {
		Fail(1, "Argument 1 must be a Docker image to base the service on\n")
	}
	ids, err := NewContainerLocators(args[1:]...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpInstallContainerRequest{
				InstallContainerRequest: jobs.InstallContainerRequest{
					RequestIdentifier: jobs.NewRequestIdentifier(),

					Id:               on.(ResourceLocator).Identifier(),
					Image:            imageId,
					Started:          start,
					Isolate:          isolate,
					Fork:             fork,
					SocketActivation: sockAct,

					Ports:        *portPairs.Get().(*containers.PortPairs),
					Environment:  &environment.Description,
					NetworkLinks: *networkLinks.NetworkLinks,
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemdAndData,
	}.StreamAndExit()
}

func buildImage(cmd *cobra.Command, args []string) {
	if err := environment.ExtractVariablesFrom(&args, false); err != nil {
		Fail(1, err.Error())
	}

	if len(args) < 3 {
		Fail(1, "Valid arguments: <source> <build image> <tag> ...\n")
	}

	buildReq.Source = args[0]
	buildReq.BaseImage = args[1]
	buildReq.Tag = args[2]
	buildReq.Writer = os.Stdout
	buildReq.DockerSocket = conf.Docker.Socket
	buildReq.Environment = environment.Description.Map()

	if buildReq.WorkingDir == "tempdir" {
		var err error
		buildReq.WorkingDir, err = ioutil.TempDir("", "sti")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		defer os.Remove(buildReq.WorkingDir)
	}

	res, err := sti.Build(buildReq)
	if err != nil {
		fmt.Printf("An error occured: %s\n", err.Error())
		return
	}

	for _, message := range res.Messages {
		fmt.Println(message)
	}
}

func setEnvironment(cmd *cobra.Command, args []string) {
	if err := environment.ExtractVariablesFrom(&args, false); err != nil {
		Fail(1, err.Error())
	}

	if len(args) < 1 {
		Fail(1, "Valid arguments: <name>... <key>=<value>...\n")
	}

	ids, err := NewContainerLocators(args[0:]...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			environment.Description.Id = on.(ResourceLocator).Identifier()
			if resetEnv {
				return &http.HttpPutEnvironmentRequest{
					PutEnvironmentRequest: jobs.PutEnvironmentRequest{environment.Description},
				}
			}
			return &http.HttpPatchEnvironmentRequest{
				PatchEnvironmentRequest: jobs.PatchEnvironmentRequest{environment.Description},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemdAndData,
	}.StreamAndExit()
}

func showEnvironment(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewContainerLocators(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid environment ids: %s\n", err.Error())
	}

	data, errors := Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpContentRequest{
				ContentRequest: jobs.ContentRequest{
					Locator: string(on.(ResourceLocator).Identifier()),
					Type:    jobs.ContentTypeEnvironment,
				},
			}
		},
		LocalInit: needsData,
		Output:    os.Stdout,
	}.Gather()

	for i := range data {
		if buf, ok := data[i].(*bytes.Buffer); ok {
			buf.WriteTo(os.Stdout)
		}
	}
	if len(errors) > 0 {
		for i := range errors {
			fmt.Fprintf(os.Stderr, "Error: %s\n", errors[i])
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func deleteContainer(cmd *cobra.Command, args []string) {
	if err := deployment.ExtractContainerLocatorsFromDeployment(deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewContainerLocators(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpDeleteContainerRequest{
				Label: on.Identity(),
				DeleteContainerRequest: jobs.DeleteContainerRequest{
					Id: on.(ResourceLocator).Identifier(),
				},
			}
		},
		Output: os.Stdout,
		OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
			fmt.Fprintf(w, "Deleted %s", job.(*http.HttpDeleteContainerRequest).Label)
		},
		LocalInit: needsSystemdAndData,
	}.StreamAndExit()
}

func linkContainers(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewContainerLocators(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}
	if networkLinks.NetworkLinks == nil {
		networkLinks.NetworkLinks = &containers.NetworkLinks{}
	}

	Executor{
		On: ids,
		Group: func(on ...Locator) jobs.Job {
			links := &jobs.ContainerLinks{make([]jobs.ContainerLink, 0, len(on))}
			buf := bytes.Buffer{}
			for i := range on {
				links.Links = append(links.Links, jobs.ContainerLink{on[i].(ResourceLocator).Identifier(), *networkLinks.NetworkLinks})
				if i > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(on[i].Identity())
			}
			return &http.HttpLinkContainersRequest{
				Label: buf.String(),
				LinkContainersRequest: jobs.LinkContainersRequest{links},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsData,
		OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
			fmt.Fprintf(w, "Links set on %s\n", job.(*http.HttpLinkContainersRequest).Label)
		},
	}.StreamAndExit()
}

func startContainer(cmd *cobra.Command, args []string) {
	if err := deployment.ExtractContainerLocatorsFromDeployment(deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewContainerLocators(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also control this container via 'systemctl start %s'\n", ids[0].(ResourceLocator).Identifier().UnitNameFor())
	}
	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpStartContainerRequest{
				StartedContainerStateRequest: jobs.StartedContainerStateRequest{
					Id: on.(ResourceLocator).Identifier(),
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
	}.StreamAndExit()
}

func stopContainer(cmd *cobra.Command, args []string) {
	if err := deployment.ExtractContainerLocatorsFromDeployment(deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewContainerLocators(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also control this container via 'systemctl stop %s'\n", ids[0].(ResourceLocator).Identifier().UnitNameFor())
	}
	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpStopContainerRequest{
				StoppedContainerStateRequest: jobs.StoppedContainerStateRequest{
					Id: on.(ResourceLocator).Identifier(),
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
	}.StreamAndExit()
}

func restartContainer(cmd *cobra.Command, args []string) {
	if err := deployment.ExtractContainerLocatorsFromDeployment(deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewContainerLocators(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also control this container via 'systemctl restart %s'\n", ids[0].(ResourceLocator).Identifier().UnitNameFor())
	}
	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpRestartContainerRequest{
				RestartContainerRequest: jobs.RestartContainerRequest{
					Id: on.(ResourceLocator).Identifier(),
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
	}.StreamAndExit()
}

func containerStatus(cmd *cobra.Command, args []string) {
	if err := deployment.ExtractContainerLocatorsFromDeployment(deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewContainerLocators(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also display the status of this container via 'systemctl status %s'\n", ids[0].(ResourceLocator).Identifier().UnitNameFor())
	}
	data, errors := Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpContainerStatusRequest{
				ContainerStatusRequest: jobs.ContainerStatusRequest{
					Id: on.(ResourceLocator).Identifier(),
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
	}.Gather()

	for i := range data {
		if buf, ok := data[i].(*bytes.Buffer); ok {
			if i > 0 {
				fmt.Fprintf(os.Stdout, "\n-------------\n")
			}
			buf.WriteTo(os.Stdout)
		}
	}
	if len(errors) > 0 {
		for i := range errors {
			fmt.Fprintf(os.Stderr, "Error: %s\n", errors[i])
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func listUnits(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		args = []string{LocalHostName}
	}
	ids, err := NewHostLocators(args...)
	if err != nil {
		Fail(1, "You must pass zero or more valid host names (use '%s' or pass no arguments for the current server): %s\n", LocalHostName, err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also display the set of containers via 'systemctl list-units'\n")
	}
	data, errors := Executor{
		On: ids,
		Group: func(on ...Locator) jobs.Job {
			return &http.HttpListContainersRequest{
				Label: string(on[0].HostIdentity()),
				ListContainersRequest: jobs.ListContainersRequest{},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
	}.Gather()

	combined := http.ListContainersResponse{}
	for i := range data {
		log.Printf("local execute %+v", reflect.TypeOf(data[i]))
		if r, ok := data[i].(*http.ListContainersResponse); ok {
			combined.Append(&r.ListContainersResponse)
		} else if j, ok := data[i].(*jobs.ListContainersResponse); ok {
			combined.Append(j)
		}
	}
	combined.Sort()
	combined.WriteTableTo(os.Stdout)
	if len(errors) > 0 {
		for i := range errors {
			fmt.Fprintf(os.Stderr, "Error: %s\n", errors[i])
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func sshKeysAdd(cmd *cobra.Command, args []string) {
	// validate that arguments for locators are passsed
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}
	// args... are locators for repositories or containers
	ids, err := NewGenericLocators(ResourceTypeContainer, args...)
	if err != nil {
		Fail(1, "You must pass 1 or more valid names: %s", err.Error())
	}
	for i := range ids {
		switch ids[i].ResourceType() {
		case ResourceTypeContainer, ResourceTypeRepository:
		default:
			Fail(1, "Only repositories or containers may be specified")
		}
	}

	keys, err := ReadAuthorizedKeysFile(keyFile)
	if err != nil {
		Fail(1, "Unable to read authorized keys file: %s", err.Error())
	}

	Executor{
		On: ids,
		Group: func(on ...Locator) jobs.Job {
			var (
				r []jobs.RepositoryPermission
				c []jobs.ContainerPermission
			)
			for i := range on {
				id := on[i].(ResourceLocator).Identifier()
				switch on[i].ResourceType() {
				case ResourceTypeContainer:
					c = append(c, jobs.ContainerPermission{id})
				case ResourceTypeRepository:
					r = append(r, jobs.RepositoryPermission{id, writeAccess})
				}
			}

			return &http.HttpCreateKeysRequest{
				CreateKeysRequest: jobs.CreateKeysRequest{
					&jobs.ExtendedCreateKeysData{
						Keys:         keys,
						Repositories: r,
						Containers:   c,
					},
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsData,
	}.StreamAndExit()
}

func createToken(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		Fail(1, "Valid arguments: <type> <content_id>")
	}

	if keyPath == "" {
		Fail(1, "You must specify --key-path to create a token")
	}
	config, err := encrypted.NewTokenConfiguration(filepath.Join(keyPath, "client"), filepath.Join(keyPath, "server.pub"))
	if err != nil {
		Fail(1, "Unable to load token configuration: %s", err.Error())
	}

	job := &jobs.ContentRequest{Locator: args[1], Type: args[0]}
	value, err := config.Sign(job, "key", expiresAt)
	if err != nil {
		Fail(1, "Unable to sign this request: %s", err.Error())
	}
	fmt.Printf("%s", value)
	os.Exit(0)
}

func initGear(cmd *cobra.Command, args []string) {
	if len(args) != 2 || !(pre || post) || (pre && post) {
		Fail(1, "Valid arguments: <id> <image_name> (--pre|--post)\n")
	}
	containerId, err := containers.NewIdentifier(args[0])
	if err != nil {
		Fail(1, "Argument 1 must be a valid gear identifier: %s\n", err.Error())
	}

	switch {
	case pre:
		if err := containers.InitPreStart(conf.Docker.Socket, containerId, args[1]); err != nil {
			Fail(2, "Unable to initialize container %s\n", err.Error())
		}
	case post:
		if err := containers.InitPostStart(conf.Docker.Socket, containerId); err != nil {
			Fail(2, "Unable to initialize container %s\n", err.Error())
		}
	}
}

func initRepository(cmd *cobra.Command, args []string) {
	if len(args) < 1 || len(args) > 2 {
		Fail(1, "Valid arguments: <repo_id> [<repo_url>]\n")
	}

	repoId, err := containers.NewIdentifier(args[0])
	if err != nil {
		Fail(1, "Argument 1 must be a valid repository identifier: %s\n", err.Error())
	}

	repoUrl := ""
	if len(args) == 2 {
		repoUrl = args[1]
	}

	needsSystemd()
	if err := gitjobs.InitializeRepository(git.RepoIdentifier(repoId), repoUrl); err != nil {
		Fail(2, "Unable to initialize repository %s\n", err.Error())
	}
}

func genAuthKeys(cmd *cobra.Command, args []string) {
	if len(args) > 1 {
		Fail(1, "Valid arguments: [<id>]\n")
	}

	var (
		u           *user.User
		err         error
		containerId containers.Identifier
		repoId      git.RepoIdentifier
		isRepo      bool
	)

	if len(args) == 1 {
		containerId, err = containers.NewIdentifier(args[0])
		if err != nil {
			Fail(1, "Argument 1 must be a valid gear identifier: %s\n", err.Error())
		}
		if gitKeys {
			repoId = git.RepoIdentifier(containerId)
			u, err = user.Lookup(repoId.LoginFor())
		} else {
			u, err = user.Lookup(containerId.LoginFor())
		}

		if err != nil {
			Fail(2, "Unable to lookup user: %s", err.Error())
		}
		isRepo = gitKeys
	} else {
		if u, err = user.LookupId(strconv.Itoa(os.Getuid())); err != nil {
			Fail(2, "Unable to lookup user")
		}
		isRepo = u.Name == "Repository user"
		if isRepo {
			repoId, err = git.NewIdentifierFromUser(u)
			if err != nil {
				Fail(1, "Not a repo user: %s\n", err.Error())
			}
		} else {
			containerId, err = containers.NewIdentifierFromUser(u)
			if err != nil {
				Fail(1, "Not a gear user: %s\n", err.Error())
			}
		}
	}

	if isRepo {
		if err := git.GenerateAuthorizedKeys(repoId, u, false, false); err != nil {
			Fail(2, "Unable to generate authorized_keys file: %s\n", err.Error())
		}
	} else {
		if err := containers.GenerateAuthorizedKeys(containerId, u, false, false); err != nil {
			Fail(2, "Unable to generate authorized_keys file: %s", err.Error())
		}
	}
}
