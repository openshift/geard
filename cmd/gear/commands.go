package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/openshift/docker-source-to-images/go"
	"github.com/openshift/geard/cleanup"
	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/deployment"
	"github.com/openshift/geard/dispatcher"
	"github.com/openshift/geard/encrypted"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/systemd"
	"github.com/openshift/geard/transport"
	"github.com/spf13/cobra"
)

var (
	pre    bool
	post   bool
	follow bool

	resetEnv bool

	start   bool
	isolate bool
	sockAct bool

	keyPath   string
	expiresAt int64

	environment  EnvironmentDescription
	portPairs    PortPairs
	networkLinks = NetworkLinks{}

	gitKeys     bool
	gitRepoName string
	gitRepoURL  string

	deploymentPath string

	buildReq    sti.BuildRequest
	keyFile     string
	writeAccess bool
	hostIp      string

	listenAddr string

	dryRun bool
	repair bool

	defaultTransport transport.TransportFlag
)

var conf = http.HttpConfiguration{
	Dispatcher: &dispatcher.Dispatcher{
		QueueFast:         10,
		QueueSlow:         1,
		Concurrent:        2,
		TrackDuplicateIds: 1000,
	},
}

var (
	needsSystemd        = LocalInitializers(systemd.Start)
	needsSystemdAndData = LocalInitializers(systemd.Start, containers.InitializeData)
	needsData           = LocalInitializers(containers.InitializeData)
)

func init() {
	defaultTransport.Set("http")
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
	gearCmd.PersistentFlags().BoolVar(&(config.SystemDockerFeatures.EnvironmentFile), "has-env-file", false, "(experimental) Use --env-file with Docker, requires master from Apr 1st")
	gearCmd.PersistentFlags().BoolVar(&(config.SystemDockerFeatures.ForegroundRun), "has-foreground", false, "(experimental) Use --foreground with Docker, requires alexlarsson/forking-run")
	gearCmd.PersistentFlags().StringVar(&deploymentPath, "with", "", "Provide a deployment descriptor to operate on")
	gearCmd.PersistentFlags().Var(&defaultTransport, "transport", "Specify an alternate mechanism to connect to the gear agent")

	deployCmd := &cobra.Command{
		Use:   "deploy <file> <host>...",
		Short: "Deploy a set of containers to the named hosts",
		Long:  "Given a simple description of a group of containers, wire them together using the gear primitives.",
		Run:   deployContainers,
	}
	deployCmd.Flags().BoolVar(&isolate, "isolate", false, "Use an isolated container running as a user")
	AddCommand(gearCmd, deployCmd, false)

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
	installImageCmd.Flags().StringVar(&environment.Path, "env-file", "", "Path to an environment file to load")
	installImageCmd.Flags().StringVar(&environment.Description.Source, "env-url", "", "A url to download environment files from")
	installImageCmd.Flags().StringVar((*string)(&environment.Description.Id), "env-id", "", "An optional identifier for the environment being set")
	AddCommand(gearCmd, installImageCmd, false)

	deleteCmd := &cobra.Command{
		Use:   "delete <name>...",
		Short: "Delete an installed container",
		Long:  "Deletes one or more installed containers from the system.  Will not clean up unused images.",
		Run:   deleteContainer,
	}
	AddCommand(gearCmd, deleteCmd, false)

	buildCmd := &cobra.Command{
		Use:   "build <source> <image> <tag> [<env>]",
		Short: "(Local) Build a new image on this host",
		Long:  "Build a new Docker image named <tag> from a source repository and base image.",
		Run:   buildImage,
	}
	buildCmd.Flags().BoolVar(&(buildReq.Clean), "clean", false, "Perform a clean build")
	buildCmd.Flags().StringVar(&(buildReq.WorkingDir), "dir", "tempdir", "Directory where generated Dockerfiles and other support scripts are created")
	buildCmd.Flags().StringVarP(&(buildReq.Method), "method", "m", "run", "Specify a method to build with. build -> 'docker build', run -> 'docker run'")
	buildCmd.Flags().StringVarP(&(buildReq.Ref), "ref", "r", "", "Specify a ref to check-out")
	buildCmd.Flags().BoolVar(&(buildReq.Verbose), "verbose", false, "Enable verbose output")
	buildCmd.Flags().StringVar(&(buildReq.CallbackUrl), "callbackUrl", "", "Specify a URL to invoke via HTTP POST upon build completion")
	buildCmd.Flags().StringVar(&environment.Path, "env-file", "", "Path to an environment file to load")
	buildCmd.Flags().StringVar(&environment.Description.Source, "env-url", "", "A url to download environment files from")
	AddCommand(gearCmd, buildCmd, false)

	setEnvCmd := &cobra.Command{
		Use:   "set-env <name>... [<env>]",
		Short: "Set environment variable values on servers",
		Long:  "Adds the listed environment values to the specified locations. The name is the environment id that multiple containers may reference. You can pass an environment file or key value pairs on the commandline.",
		Run:   setEnvironment,
	}
	setEnvCmd.Flags().BoolVar(&resetEnv, "reset", false, "Remove any existing values")
	setEnvCmd.Flags().StringVar(&environment.Path, "env-file", "", "Path to an environment file to load")
	AddCommand(gearCmd, setEnvCmd, false)

	envCmd := &cobra.Command{
		Use:   "env <name>...",
		Short: "Retrieve environment variable values by id",
		Long:  "Return the environment variables matching the provided ids",
		Run:   showEnvironment,
	}
	AddCommand(gearCmd, envCmd, false)

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
	AddCommand(gearCmd, startCmd, false)

	stopCmd := &cobra.Command{
		Use:   "stop <name>...",
		Short: "Invoke systemd to stop a container",
		Long:  ``,
		Run:   stopContainer,
	}
	AddCommand(gearCmd, stopCmd, false)

	restartCmd := &cobra.Command{
		Use:   "restart <name>...",
		Short: "Invoke systemd to restart a container",
		Long:  "Queues the restart and immediately returns.", //  Use -f to attach to the logs.",
		Run:   restartContainer,
	}
	//startCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Attach to the logs after startup")
	AddCommand(gearCmd, restartCmd, false)

	statusCmd := &cobra.Command{
		Use:   "status <name>...",
		Short: "Retrieve the systemd status of one or more containers",
		Long:  "Shows the equivalent of 'systemctl status ctr-<name>' for each listed unit",
		Run:   containerStatus,
	}
	AddCommand(gearCmd, statusCmd, false)

	listUnitsCmd := &cobra.Command{
		Use:   "list-units <host>...",
		Short: "Retrieve the list of services across all hosts",
		Long:  "Shows the equivalent of 'systemctl list-units ctr-<name>' for each installed container",
		Run:   listUnits,
	}
	AddCommand(gearCmd, listUnitsCmd, false)

	ExtendCommands(gearCmd, false)

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "(Local) Start the gear agent",
		Long:  "Launch the gear agent. Will not send itself to the background.",
		Run:   daemon,
	}
	daemonCmd.Flags().StringVarP(&listenAddr, "listen-address", "A", ":43273", "Set the address for the http endpoint to listen on")
	AddCommand(gearCmd, daemonCmd, true)

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "(local) Perform housekeeping tasks on geard directories",
		Long:  "Perform various tasks to clean up the state, images, directories and other resources.",
		Run:   clean,
	}
	cleanCmd.Flags().BoolVarP(&dryRun, "dry-run", "", false, "List the cleanups, but do not execute.")
	cleanCmd.Flags().BoolVarP(&repair, "repair", "", false, "Perform potentially irrecoverable cleanups.")
	AddCommand(gearCmd, cleanCmd, true)

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "(Local) Stop and disable systemd gear units",
		Long:  "Disable all registered resources from systemd to allow them to be removed from the system.  Will reload the systemd daemon config.",
		Run:   purge,
	}
	AddCommand(gearCmd, purgeCmd, true)

	initGearCmd := &cobra.Command{
		Use:   "init <name> <image>",
		Short: "(Local) Setup the environment for a container",
		Long:  "",
		Run:   initGear,
	}
	initGearCmd.Flags().BoolVarP(&pre, "pre", "", false, "Perform pre-start initialization")
	initGearCmd.Flags().BoolVarP(&post, "post", "", false, "Perform post-start initialization")
	AddCommand(gearCmd, initGearCmd, true)

	createTokenCmd := &cobra.Command{
		Use:   "create-token <type> <content_id>",
		Short: "(Local) Generate a content request token",
		Long:  "Create a URL that will serve as a content request token using a server public key and client private key.",
		Run:   createToken,
	}
	createTokenCmd.Flags().Int64Var(&expiresAt, "expires-at", time.Now().Unix()+3600, "Specify the content request token expiration time in seconds after the Unix epoch")
	gearCmd.AddCommand(createTokenCmd)

	ExtendCommands(gearCmd, true)

	if err := gearCmd.Execute(); err != nil {
		Fail(1, err.Error())
	}
}

func gear(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func purge(cmd *cobra.Command, args []string) {
	needsSystemd()
	containers.Clean()
}

func deployContainers(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <deployment_file> <host> ...")
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
		args = append(args, transport.Local.String())
	}
	servers, err := transport.NewTransportLocators(defaultTransport.Get(), args[1:]...)
	if err != nil {
		Fail(1, "You must pass zero or more valid host names (use '%s' or pass no arguments for the current server): %s", transport.Local.String(), err.Error())
	}

	re := regexp.MustCompile("\\.\\d{8}\\-\\d{6}\\z")
	now := time.Now().Format(".20060102-150405")
	base := filepath.Base(path)
	base = re.ReplaceAllString(base, "")
	newPath := base + now

	fmt.Printf("==> Deploying %s\n", path)
	changes, removed, err := deploy.Describe(deployment.SimplePlacement(servers), defaultTransport.Get())
	if err != nil {
		Fail(1, "Deployment is not valid: %s", err.Error())
	}

	if len(removed) > 0 {
		removedIds, err := LocatorsForDeploymentInstances(defaultTransport.Get(), removed)
		if err != nil {
			Fail(1, "Unable to generate deployment info: %s", err.Error())
		}

		failures := Executor{
			On: removedIds,
			Serial: func(on Locator) jobs.Job {
				return &cjobs.DeleteContainerRequest{
					Id:    AsIdentifier(on),
					Label: on.Identity(),
				}
			},
			Output: os.Stdout,
			OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
				fmt.Fprintf(w, "==> Deleted %s", job.(jobs.LabeledJob).JobLabel())
			},
			LocalInit: needsSystemdAndData,
			Transport: defaultTransport.Get(),
		}.Stream()
		for i := range failures {
			fmt.Fprintf(os.Stderr, failures[i].Error())
		}
	}

	addedIds, err := LocatorsForDeploymentInstances(defaultTransport.Get(), changes.Instances.Added())
	if err != nil {
		Fail(1, "Unable to generate deployment info: %s", err.Error())
	}

	errors := Executor{
		On: addedIds,
		Serial: func(on Locator) jobs.Job {
			instance, _ := changes.Instances.Find(AsIdentifier(on))
			links := instance.NetworkLinks()
			return &cjobs.InstallContainerRequest{
				RequestIdentifier: jobs.NewRequestIdentifier(),

				Id:      instance.Id,
				Image:   instance.Image,
				Isolate: isolate,

				Ports:        instance.Ports.PortPairs(),
				NetworkLinks: &links,
			}
		},
		OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
			installJob := job.(*cjobs.InstallContainerRequest)
			instance, _ := changes.Instances.Find(installJob.Id)
			if pairs, ok := installJob.PortMappingsFrom(r.Pending); ok {
				if !instance.Ports.Update(pairs) {
					fmt.Fprintf(os.Stderr, "Not all ports listed %+v were returned by the server %+v", instance.Ports, pairs)
				}
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemdAndData,
		Transport: defaultTransport.Get(),
	}.Stream()

	changes.UpdateLinks()

	for _, c := range changes.Containers {
		instances := c.Instances()
		if len(instances) > 0 {
			for _, link := range instances[0].NetworkLinks() {
				fmt.Printf("==> Linking %s: %s:%d -> %s:%d\n", c.Name, link.FromHost, link.FromPort, link.ToHost, link.ToPort)
			}
		}
	}

	contents, _ := json.Marshal(changes)
	if err := ioutil.WriteFile(newPath, contents, 0664); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to write %s: %s\n", newPath, err.Error())
	}

	linkedIds, err := LocatorsForDeploymentInstances(defaultTransport.Get(), changes.Instances.Linked())
	if err != nil {
		Fail(1, "Unable to generate deployment info: %s", err.Error())
	}

	Executor{
		On: linkedIds,
		Group: func(on ...Locator) jobs.Job {
			links := []cjobs.ContainerLink{}
			for i := range on {
				instance, _ := changes.Instances.Find(AsIdentifier(on[i]))
				network := instance.NetworkLinks()
				if len(network) > 0 {
					links = append(links, cjobs.ContainerLink{instance.Id, network})
				}
			}

			return &cjobs.LinkContainersRequest{&cjobs.ContainerLinks{links}, on[0].TransportLocator().String()}
		},
		Output:    os.Stdout,
		Transport: defaultTransport.Get(),
	}.Stream()

	Executor{
		On: addedIds,
		Serial: func(on Locator) jobs.Job {
			return &cjobs.StartedContainerStateRequest{
				Id: AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		Transport: defaultTransport.Get(),
	}.Stream()

	fmt.Printf("==> Deployed as %s\n", newPath)
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
		Fail(1, "Valid arguments: <image_name> <id> ...")
	}

	imageId := args[0]
	if imageId == "" {
		Fail(1, "Argument 1 must be a Docker image to base the service on")
	}
	ids, err := NewContainerLocators(defaultTransport.Get(), args[1:]...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	for _, locator := range ids {
		if imageId == string(AsIdentifier(locator)) {
			Fail(1, "Image name and container id must not be the same: %s", imageId)
		}
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			r := cjobs.InstallContainerRequest{
				RequestIdentifier: jobs.NewRequestIdentifier(),

				Id:               AsIdentifier(on),
				Image:            imageId,
				Started:          start,
				Isolate:          isolate,
				SocketActivation: sockAct,

				Ports:        *portPairs.Get().(*port.PortPairs),
				Environment:  &environment.Description,
				NetworkLinks: networkLinks.NetworkLinks,
			}
			return &r
		},
		Output:    os.Stdout,
		LocalInit: needsSystemdAndData,
		Transport: defaultTransport.Get(),
	}.StreamAndExit()
}

func buildImage(cmd *cobra.Command, args []string) {
	if err := environment.ExtractVariablesFrom(&args, false); err != nil {
		Fail(1, err.Error())
	}

	if len(args) < 3 {
		Fail(1, "Valid arguments: <source> <build image> <tag> ...")
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
		Fail(1, "Valid arguments: <name>... <key>=<value>...")
	}

	ids, err := NewContainerLocators(defaultTransport.Get(), args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			environment.Description.Id = AsIdentifier(on)
			if resetEnv {
				return &cjobs.PutEnvironmentRequest{environment.Description}
			}

			return &cjobs.PatchEnvironmentRequest{environment.Description}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemdAndData,
		Transport: defaultTransport.Get(),
	}.StreamAndExit()
}

func showEnvironment(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := NewContainerLocators(defaultTransport.Get(), args[0:]...)
	if err != nil {
		Fail(1, "You must pass one or more valid environment ids: %s", err.Error())
	}

	data, errors := Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &cjobs.ContentRequest{
				Locator: string(AsIdentifier(on)),
				Type:    cjobs.ContentTypeEnvironment,
			}
		},
		LocalInit: needsData,
		Output:    os.Stdout,
		Transport: defaultTransport.Get(),
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
	if err := ExtractContainerLocatorsFromDeployment(defaultTransport.Get(), deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}

	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}

	ids, err := NewContainerLocators(defaultTransport.Get(), args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &cjobs.DeleteContainerRequest{
				Id:    AsIdentifier(on),
				Label: on.Identity(),
			}
		},
		Output: os.Stdout,
		OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
			fmt.Fprintf(w, "Deleted %s", string(job.(*cjobs.DeleteContainerRequest).Id))
		},
		LocalInit: needsSystemdAndData,
		Transport: defaultTransport.Get(),
	}.StreamAndExit()
}

func linkContainers(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := NewContainerLocators(defaultTransport.Get(), args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}
	if networkLinks.NetworkLinks == nil {
		networkLinks.NetworkLinks = &containers.NetworkLinks{}
	}

	Executor{
		On: ids,
		Group: func(on ...Locator) jobs.Job {
			links := &cjobs.ContainerLinks{make([]cjobs.ContainerLink, 0, len(on))}
			buf := bytes.Buffer{}
			for i := range on {
				links.Links = append(links.Links, cjobs.ContainerLink{AsIdentifier(on[i]), *networkLinks.NetworkLinks})
				if i > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(on[i].Identity())
			}
			return &cjobs.LinkContainersRequest{links, buf.String()}
		},
		Output:    os.Stdout,
		LocalInit: needsData,
		OnSuccess: func(r *CliJobResponse, w io.Writer, job interface{}) {
			fmt.Fprintf(w, "Links set on %s\n", job.(jobs.LabeledJob).JobLabel())
		},
		Transport: defaultTransport.Get(),
	}.StreamAndExit()
}

func startContainer(cmd *cobra.Command, args []string) {
	if err := ExtractContainerLocatorsFromDeployment(defaultTransport.Get(), deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := NewContainerLocators(defaultTransport.Get(), args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &cjobs.StartedContainerStateRequest{
				Id: AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
		Transport: defaultTransport.Get(),
	}.StreamAndExit()
}

func stopContainer(cmd *cobra.Command, args []string) {
	if err := ExtractContainerLocatorsFromDeployment(defaultTransport.Get(), deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := NewContainerLocators(defaultTransport.Get(), args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &cjobs.StoppedContainerStateRequest{
				Id: AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
		Transport: defaultTransport.Get(),
	}.StreamAndExit()
}

func restartContainer(cmd *cobra.Command, args []string) {
	if err := ExtractContainerLocatorsFromDeployment(defaultTransport.Get(), deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := NewContainerLocators(defaultTransport.Get(), args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &cjobs.RestartContainerRequest{
				Id: AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
		Transport: defaultTransport.Get(),
	}.StreamAndExit()
}

func containerStatus(cmd *cobra.Command, args []string) {
	if err := ExtractContainerLocatorsFromDeployment(defaultTransport.Get(), deploymentPath, &args); err != nil {
		Fail(1, err.Error())
	}
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := NewContainerLocators(defaultTransport.Get(), args...)
	if err != nil {
		Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	data, errors := Executor{
		On: ids,
		Serial: func(on Locator) jobs.Job {
			return &cjobs.ContainerStatusRequest{
				Id: AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
		Transport: defaultTransport.Get(),
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
		args = []string{transport.Local.String()}
	}
	servers, err := NewHostLocators(defaultTransport.Get(), args[0:]...)
	if err != nil {
		Fail(1, "You must pass zero or more valid host names (use '%s' or pass no arguments for the current server): %s", transport.Local.String(), err.Error())
	}

	data, errors := Executor{
		On: servers,
		Group: func(on ...Locator) jobs.Job {
			return &cjobs.ListContainersRequest{on[0].TransportLocator().String()}
		},
		Output:    os.Stdout,
		LocalInit: needsSystemd,
		Transport: defaultTransport.Get(),
	}.Gather()

	combined := http.ListContainersResponse{}
	for i := range data {
		if r, ok := data[i].(*http.ListContainersResponse); ok {
			combined.Append(&r.ListContainersResponse)
		} else if j, ok := data[i].(*cjobs.ListContainersResponse); ok {
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

	job := &cjobs.ContentRequest{Locator: args[1], Type: args[0]}
	value, err := config.Sign(job, "key", expiresAt)
	if err != nil {
		Fail(1, "Unable to sign this request: %s", err.Error())
	}
	fmt.Printf("%s", value)
	os.Exit(0)
}

func initGear(cmd *cobra.Command, args []string) {
	if len(args) != 2 || !(pre || post) || (pre && post) {
		Fail(1, "Valid arguments: <id> <image_name> (--pre|--post)")
	}
	containerId, err := containers.NewIdentifier(args[0])
	if err != nil {
		Fail(1, "Argument 1 must be a valid gear identifier: %s", err.Error())
	}

	switch {
	case pre:
		if err := InitPreStart(conf.Docker.Socket, containerId, args[1]); err != nil {
			Fail(2, "Unable to initialize container %s", err.Error())
		}
	case post:
		if err := InitPostStart(conf.Docker.Socket, containerId); err != nil {
			Fail(2, "Unable to initialize container %s", err.Error())
		}
	}
}

func clean(cmd *cobra.Command, args []string) {
	logInfo := log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	logError := log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime)

	cleanup.Clean(&cleanup.CleanerContext{DryRun: dryRun, Repair: repair, LogInfo: logInfo, LogError: logError})
}
