package main

import (
	"bufio"
	"bytes"
	"code.google.com/p/go.crypto/ssh"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	nethttp "net/http"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pmorie/go-sti"
	"github.com/smarterclayton/cobra"
	. "github.com/smarterclayton/geard/cmd"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/dispatcher"
	"github.com/smarterclayton/geard/encrypted"
	"github.com/smarterclayton/geard/git"
	githttp "github.com/smarterclayton/geard/git/http"
	gitjobs "github.com/smarterclayton/geard/git/jobs"
	"github.com/smarterclayton/geard/http"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/systemd"
)

var (
	pre          bool
	post         bool
	follow       bool
	start        bool
	listenAddr   string
	resetEnv     bool
	simple       bool
	fork         bool
	sockAct      bool
	keyPath      string
	expiresAt    int64
	envString    string
	environment  EnvironmentDescription
	portPairs    PortPairs
	networkLinks NetworkLinks
	gitKeys      bool
	gitRepoName  string
	gitRepoURL   string
	buildReq     sti.BuildRequest
	keyFile      string
	writeAccess  bool
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
		Long:  "A commandline client and server that allows Docker containers to be installed to Systemd in an opinionated and distributed fashion.\n\nComplete documentation is available at http://github.com/smarterclayton/geard",
		Run:   gear,
	}
	gearCmd.PersistentFlags().StringVar(&(keyPath), "key-path", "", "Specify the directory containing the server private key and trusted client public keys")
	gearCmd.PersistentFlags().StringVarP(&(conf.Docker.Socket), "docker-socket", "S", "unix:///var/run/docker.sock", "Set the docker socket to use")

	installImageCmd := &cobra.Command{
		Use:   "install <image> <name>... <key>=<value>",
		Short: "Install a docker image as a systemd service",
		Long:  "Install a docker image as one or more systemd services on one or more servers.\n\nSpecify a location on a remote server with <host>[:<port>]/<name> instead of <name>.  The default port is 2223.",
		Run:   installImage,
	}
	installImageCmd.Flags().VarP(&portPairs, "ports", "p", "List of comma separated port pairs to bind '<internal>:<external>,...'. Use zero to request a port be assigned.")
	installImageCmd.Flags().VarP(&networkLinks, "net-links", "n", "List of comma separated port pairs to wire '<local_port>:<host>:<remote_port>,...'. Host and remote port may be empty.")
	installImageCmd.Flags().BoolVar(&start, "start", false, "Start the container immediately")
	installImageCmd.Flags().BoolVar(&simple, "simple", false, "Use a simple container (experimental)")
	installImageCmd.Flags().BoolVar(&fork, "fork", false, "Use a forked container (experimental, requires docker branch)")
	installImageCmd.Flags().BoolVar(&sockAct, "socket-activated", false, "Use a socket-activated container (experimental, requires docker branch)")
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
		Use:   "build <source> <build_image> <app_image_tag>",
		Short: "Build an image",
		Long:  "Build an image",
		Run: func(cmd *cobra.Command, args []string) {
			buildReq.Source = args[0]
			buildReq.BaseImage = args[1]
			buildReq.Tag = args[2]
			buildReq.Writer = os.Stdout

			envs, _ := parseEnvs(envString)
			buildReq.Environment = envs

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
		},
	}
	buildCmd.Flags().BoolVar(&(buildReq.Clean), "clean", false, "Perform a clean build")
	buildCmd.Flags().StringVar(&(buildReq.WorkingDir), "dir", "tempdir", "Directory where generated Dockerfiles and other support scripts are created")
	buildCmd.Flags().StringVarP(&(buildReq.RuntimeImage), "runtime", "R", "", "Set the runtime image to use")
	buildCmd.Flags().StringVarP(&envString, "env", "e", "", "Specify an environment var NAME=VALUE,NAME2=VALUE2,...")
	buildCmd.Flags().StringVarP(&(buildReq.Method), "method", "m", "build", "Specify a method to build with. build -> 'docker build', run -> 'docker run'")
	buildCmd.Flags().StringVarP(&(buildReq.DockerSocket), "url", "U", "unix:///var/run/docker.sock", "Set the url of the docker socket to use")
	buildCmd.Flags().BoolVar(&(buildReq.Debug), "debug", false, "Enable debugging output")
	gearCmd.AddCommand(buildCmd)

	setEnvCmd := &cobra.Command{
		Use:   "set-env <name>... <key>=<value>...",
		Short: "Set environment variable values on servers",
		Long:  "Adds the listed environment values to the specified locations. The name is the environment id that multiple containers may reference.",
		Run:   setEnvironment,
	}
	setEnvCmd.Flags().BoolVar(&resetEnv, "reset", false, "Remove any existing values")
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
	linkCmd.Flags().VarP(&networkLinks, "net-links", "n", "List of comma separated port pairs to wire '<local_port>:<host>:<remote_port>,...'. Host and remote port may be empty.")
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

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "(Local) Start the gear server",
		Long:  "Launch the gear HTTP API server as a daemon. Will not send itself to the background.",
		Run:   daemon,
	}
	daemonCmd.Flags().StringVarP(&listenAddr, "listen-address", "A", ":8080", "Set the address for the http endpoint to listen on")
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

	sshKeysCmd := &cobra.Command{
		Use:   "keys",
		Short: "Add a public key to enable SSH access to a repository or container location",
		Long:  "Add a public key to enable SSH access to a repository or container location.",
		Run:   sshKeysAdd,
	}
	sshKeysCmd.Flags().BoolVar(&writeAccess, "write", false, "True if write access is provided for this key to the repository")
	sshKeysCmd.Flags().StringVar(&keyFile, "key-file", "", "read input from FILE specified matching sshd AuthorizedKeysFile format")
	gearCmd.AddCommand(sshKeysCmd)

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
	ids, err := NewRemoteIdentifiers(args[1:]...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpInstallContainerRequest{
				InstallContainerRequest: jobs.InstallContainerRequest{
					RequestIdentifier: jobs.NewRequestIdentifier(),

					Id:               on.(*RemoteIdentifier).Id,
					Image:            imageId,
					Started:          start,
					Simple:           simple,
					Fork:             fork,
					SocketActivation: sockAct,

					Ports:        *portPairs.Get().(*containers.PortPairs),
					Environment:  &environment.Description,
					NetworkLinks: networkLinks.NetworkLinks,
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemdAndData,
	}.StreamAndExit()
}

func setEnvironment(cmd *cobra.Command, args []string) {
	if err := environment.ExtractVariablesFrom(&args, false); err != nil {
		Fail(1, err.Error())
	}

	if len(args) < 1 {
		Fail(1, "Valid arguments: <name>... <key>=<value>...\n")
	}

	ids, err := NewRemoteIdentifiers(args[0:]...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			environment.Description.Id = on.(*RemoteIdentifier).Id
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
	ids, err := NewRemoteIdentifiers(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid environment ids: %s\n", err.Error())
	}

	data, errors := Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpContentRequest{
				ContentRequest: jobs.ContentRequest{
					Locator: string(on.(*RemoteIdentifier).Id),
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
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewRemoteIdentifiers(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpDeleteContainerRequest{
				Label: on.(*RemoteIdentifier).String(),
				DeleteContainerRequest: jobs.DeleteContainerRequest{
					Id: on.(*RemoteIdentifier).Id,
				},
			}
		},
		Output: os.Stdout,
		OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
			fmt.Fprintf(w, "Deleted %s", job.(*http.HttpDeleteContainerRequest).Label)
		},
		LocalInit: needsData,
	}.StreamAndExit()
}

func linkContainers(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewRemoteIdentifiers(args...)
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
			for i := range on {
				links.Links = append(links.Links, jobs.ContainerLink{on[i].(*RemoteIdentifier).Id, *networkLinks.NetworkLinks})
			}
			return &http.HttpLinkContainersRequest{
				Label: on[0].String(),
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
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewRemoteIdentifiers(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also control this container via 'systemctl start %s'\n", ids[0].(*RemoteIdentifier).Id.UnitNameFor())
	}
	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpStartContainerRequest{
				StartedContainerStateRequest: jobs.StartedContainerStateRequest{
					Id: on.(*RemoteIdentifier).Id,
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
	}.StreamAndExit()
}

func stopContainer(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewRemoteIdentifiers(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also control this container via 'systemctl stop %s'\n", ids[0].(*RemoteIdentifier).Id.UnitNameFor())
	}
	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpStopContainerRequest{
				StoppedContainerStateRequest: jobs.StoppedContainerStateRequest{
					Id: on.(*RemoteIdentifier).Id,
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
	}.StreamAndExit()
}

func restartContainer(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewRemoteIdentifiers(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also control this container via 'systemctl restart %s'\n", ids[0].(*RemoteIdentifier).Id.UnitNameFor())
	}
	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpRestartContainerRequest{
				RestartContainerRequest: jobs.RestartContainerRequest{
					Id: on.(*RemoteIdentifier).Id,
				},
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
	}.StreamAndExit()
}

func containerStatus(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...\n")
	}
	ids, err := NewRemoteIdentifiers(args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s\n", err.Error())
	}

	if len(ids) == 1 && !ids[0].IsRemote() {
		fmt.Fprintf(os.Stderr, "You can also display the status of this container via 'systemctl status %s'\n", ids[0].(*RemoteIdentifier).Id.UnitNameFor())
	}
	data, errors := Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &http.HttpContainerStatusRequest{
				ContainerStatusRequest: jobs.ContainerStatusRequest{
					Id: on.(*RemoteIdentifier).Id,
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
	ids, err := NewRemoteHostIdentifiers(args...)
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
				Label: string(on[0].(*RemoteIdentifier).Host),
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
			Fail(2, "Unable to generate authorized_keys file: %s\n", err.Error())
		}
	}
}

func parseEnvs(envStr string) (map[string]string, error) {
	if envStr == "" {
		return nil, nil
	}

	var envs map[string]string
	pairs := strings.Split(envStr, ",")

	for _, pair := range pairs {
		atoms := strings.Split(pair, "=")

		if len(atoms) != 2 {
			return nil, errors.New("Malformed env string: " + pair)
		}

		name := atoms[0]
		value := atoms[1]

		envs[name] = value
	}

	return envs, nil
}

func sshKeysAdd(cmd *cobra.Command, args []string) {

	var (
		data []byte
		keys []jobs.KeyData
		err  error
	)

	// validate that arguments for locators are passsed
	if len(args) < 1 {
		Fail(1, "Valid arguments: [LOCATOR] ...\n")
	}
	// args... are locators for repositories or containers
	ids, err := NewRemoteIdentifiers(args...)
	if err != nil {
		Fail(1, "You must pass 1 or more valid LOCATOR names: %s\n", err.Error())
	}

	// keyFile - contains the sshd AuthorizedKeysFile location
	// Stdin - contains the AuthorizedKeysFile if keyFile is not specified
	if len(keyFile) != 0 {
		absPath, _ := filepath.Abs(keyFile)
		data, err = ioutil.ReadFile(absPath)
		if err != nil {
			Fail(1, "You must pass a valid FILE that exists.\n%v", err.Error())
		}
	} else {
		data, _ = ioutil.ReadAll(os.Stdin)
	}

	bytesReader := bytes.NewReader(data)
	scanner := bufio.NewScanner(bytesReader)
	for scanner.Scan() {
		// Parse the AuthorizedKeys line
		pk, _, _, _, ok := ssh.ParseAuthorizedKey(scanner.Bytes())
		if !ok {
			Fail(1, "Unable to parse authorized key from input")
		}
		value := ssh.MarshalAuthorizedKey(pk)
		keys = append(keys, jobs.KeyData{pk.PublicKeyAlgo(), string(value)})
	}

	Executor{
		On: ids,
		Group: func(on ...Locator) jobs.Job {
			var (
				r []jobs.RepositoryPermission
				c []jobs.ContainerPermission
			)
			for _, loc := range on {
				cId, _ := containers.NewIdentifier(loc.String())
				if loc.ResourceType() == ResourceTypeContainer {
					c = append(c, jobs.ContainerPermission{cId})
				} else if loc.ResourceType() == ResourceTypeRepository {
					r = append(r, jobs.RepositoryPermission{cId, writeAccess})
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
		Output: os.Stdout,
	}.StreamAndExit()
}
