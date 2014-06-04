package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	gcmd "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/config"
	"github.com/openshift/geard/containers"
	cjobs "github.com/openshift/geard/containers/jobs"
	"github.com/openshift/geard/deployment"
	"github.com/openshift/geard/dispatcher"
	"github.com/spf13/cobra"
	// "github.com/openshift/geard/encrypted"
	"github.com/openshift/geard/http"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/sti"
	"github.com/openshift/geard/transport"
)

var (
	follow bool

	resetEnv bool

	start   bool
	isolate bool
	sockAct bool

	keyPath   string
	expiresAt int64

	environment  gcmd.EnvironmentDescription
	portPairs    gcmd.PortPairs
	networkLinks = gcmd.NetworkLinks{}

	gitKeys     bool
	gitRepoName string
	gitRepoURL  string

	deploymentPath string

	keyFile     string
	writeAccess bool
	hostIp      string

	insecure   bool
	timeout    int64
	listenAddr string

	defaultTransport LocalTransportFlag

	version string
)

var buildReq = &sti.STIRequest{}

var conf = http.HttpConfiguration{
	Dispatcher: &dispatcher.Dispatcher{
		QueueFast:         10,
		QueueSlow:         1,
		Concurrent:        2,
		TrackDuplicateIds: 1000,
	},
}

func init() {
	log.SetFlags(0)
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
	gearCmd.PersistentFlags().BoolVar(&(config.SystemDockerFeatures.EnvironmentFile), "has-env-file", true, "Use --env-file with Docker, set false if older than 0.11")
	gearCmd.PersistentFlags().BoolVar(&(config.SystemDockerFeatures.ForegroundRun), "has-foreground", false, "(experimental) Use --foreground with Docker, requires alexlarsson/forking-run")
	gearCmd.PersistentFlags().StringVar(&deploymentPath, "with", "", "Provide a deployment descriptor to operate on")
	gearCmd.PersistentFlags().Var(&defaultTransport, "transport", "Specify an alternate mechanism to connect to the gear agent")
	gearCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "k", false, "Do not verify CA certificate on SSL connections and transfers")

	deployCmd := &cobra.Command{
		Use:   "deploy <file|url> <host>...",
		Short: "Deploy a set of containers to the named hosts",
		Long:  "Given a simple description of a group of containers, wire them together using the gear primitives.",
		Run:   deployContainers,
	}
	deployCmd.Flags().BoolVar(&isolate, "isolate", false, "Use an isolated container running as a user")
	deployCmd.Flags().Int64VarP(&timeout, "timeout", "", 300, "Number of seconds to wait for HTTP/S server")
	gcmd.AddCommand(gearCmd, deployCmd, false)

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
	gcmd.AddCommand(gearCmd, installImageCmd, false)

	deleteCmd := &cobra.Command{
		Use:   "delete <name>...",
		Short: "Delete an installed container",
		Long:  "Deletes one or more installed containers from the system.  Will not clean up unused images.",
		Run:   deleteContainer,
	}
	gcmd.AddCommand(gearCmd, deleteCmd, false)

	buildCmd := &cobra.Command{
		Use:   "build <source> <image> <tag> [<env>]",
		Short: "(Local) Build a new image on this host",
		Long:  "Build a new Docker image named <tag> from a source repository and base image.",
		Run:   buildImage,
	}
	buildCmd.Flags().BoolVar(&(buildReq.Clean), "clean", false, "Perform a clean build")
	buildCmd.Flags().StringVarP(&(buildReq.Ref), "ref", "r", "", "Specify a ref to check-out")
	buildCmd.Flags().BoolVar(&(buildReq.Verbose), "verbose", false, "Enable verbose output")
	buildCmd.Flags().StringVar(&(buildReq.CallbackUrl), "callbackUrl", "", "Specify a URL to invoke via HTTP POST upon build completion")
	buildCmd.Flags().StringVar(&environment.Path, "env-file", "", "Path to an environment file to load")
	buildCmd.Flags().StringVar(&environment.Description.Source, "env-url", "", "A url to download environment files from")
	buildCmd.Flags().StringVarP(&(buildReq.ScriptsUrl), "scripts", "s", "", "Specify a URL for the assemble and run scripts")
	gcmd.AddCommand(gearCmd, buildCmd, false)

	setEnvCmd := &cobra.Command{
		Use:   "set-env <name>... [<env>]",
		Short: "Set environment variable values on servers",
		Long:  "Adds the listed environment values to the specified locations. The name is the environment id that multiple containers may reference. You can pass an environment file or key value pairs on the commandline.",
		Run:   setEnvironment,
	}
	setEnvCmd.Flags().BoolVar(&resetEnv, "reset", false, "Remove any existing values")
	setEnvCmd.Flags().StringVar(&environment.Path, "env-file", "", "Path to an environment file to load")
	gcmd.AddCommand(gearCmd, setEnvCmd, false)

	envCmd := &cobra.Command{
		Use:   "env <name>...",
		Short: "Retrieve environment variable values by id",
		Long:  "Return the environment variables matching the provided ids",
		Run:   showEnvironment,
	}
	gcmd.AddCommand(gearCmd, envCmd, false)

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
	gcmd.AddCommand(gearCmd, startCmd, false)

	stopCmd := &cobra.Command{
		Use:   "stop <name>...",
		Short: "Invoke systemd to stop a container",
		Long:  ``,
		Run:   stopContainer,
	}
	gcmd.AddCommand(gearCmd, stopCmd, false)

	restartCmd := &cobra.Command{
		Use:   "restart <name>...",
		Short: "Invoke systemd to restart a container",
		Long:  "Queues the restart and immediately returns.", //  Use -f to attach to the logs.",
		Run:   restartContainer,
	}
	//startCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Attach to the logs after startup")
	gcmd.AddCommand(gearCmd, restartCmd, false)

	statusCmd := &cobra.Command{
		Use:   "status <name>...",
		Short: "Retrieve the systemd status of one or more containers",
		Long:  "Shows the equivalent of 'systemctl status ctr-<name>' for each listed unit",
		Run:   containerStatus,
	}
	gcmd.AddCommand(gearCmd, statusCmd, false)

	listUnitsCmd := &cobra.Command{
		Use:   "list-units <host>...",
		Short: "Retrieve the list of services across all hosts",
		Long:  "Shows the equivalent of 'systemctl list-units ctr-<name>' for each installed container",
		Run:   listUnits,
	}
	gcmd.AddCommand(gearCmd, listUnitsCmd, false)

	gcmd.ExtendCommands(gearCmd, false)

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "(Local) Start the gear agent",
		Long:  "Launch the gear agent. Will not send itself to the background.",
		Run:   daemon,
	}
	daemonCmd.Flags().StringVarP(&listenAddr, "listen-address", "A", ":43273", "Set the address for the http endpoint to listen on")
	gcmd.AddCommand(gearCmd, daemonCmd, true)

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Stop and disable all containers",
		Long:  "Disable all registered resources from systemd to allow them to be removed from the system.  Will reload the systemd daemon config.",
		Run:   purge,
	}
	gcmd.AddCommand(gearCmd, purgeCmd, true)

	// createTokenCmd := &cobra.Command{
	// 	Use:   "create-token <type> <content_id>",
	// 	Short: "(Local) Generate a content request token",
	// 	Long:  "Create a URL that will serve as a content request token using a server public key and client private key.",
	// 	Run:   createToken,
	// }
	// createTokenCmd.Flags().Int64Var(&expiresAt, "expires-at", time.Now().Unix()+3600, "Specify the content request token expiration time in seconds after the Unix epoch")
	// gearCmd.gcmd.AddCommand(createTokenCmd)

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Long:  "Display version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gear %s\n", version)
		},
	}

	gearCmd.AddCommand(versionCmd)

	gcmd.ExtendCommands(gearCmd, true)

	if err := gearCmd.Execute(); err != nil {
		gcmd.Fail(1, err.Error())
	}
}

func gear(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func deployContainers(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <deployment_file|URL> <host> ...")
	}

	t := defaultTransport.Get()

	path := args[0]
	if path == "" {
		gcmd.Fail(1, "Argument 1 must be deployment file or URL describing how the containers are related")
	}

	u, err := url.Parse(path)
	if nil != err {
		gcmd.Fail(1, "Cannot Parse Argument 1: %s", err.Error())
	}

	var deploy *deployment.Deployment
	switch u.Scheme {
	case "":
		deploy, err = deployment.NewDeploymentFromFile(u.Path)
	case "file":
		deploy, err = deployment.NewDeploymentFromFile(u.Path)
	case "http", "https":
		deploy, err = deployment.NewDeploymentFromURL(u.String(), insecure, time.Duration(timeout))
	default:
		gcmd.Fail(1, "Unsupported URL Scheme '%s' for deployment", u.Scheme)
	}

	if nil != err {
		gcmd.Fail(1, "Unable to load deployment from %s: %s", path, err.Error())
	}

	if len(args) == 1 {
		args = append(args, transport.Local.String())
	}
	servers, err := transport.NewTransportLocators(t, args[1:]...)
	if err != nil {
		gcmd.Fail(1, "You must pass zero or more valid host names (use '%s' or pass no arguments for the current server): %s", transport.Local.String(), err.Error())
	}

	re := regexp.MustCompile("\\.\\d{8}\\-\\d{6}\\z")
	now := time.Now().Format(".20060102-150405")
	base := filepath.Base(path)
	base = re.ReplaceAllString(base, "")
	newPath := base + now

	fmt.Printf("==> Deploying %s\n", path)
	changes, removed, err := deploy.Describe(deployment.SimplePlacement(servers), t)
	if err != nil {
		gcmd.Fail(1, "Deployment is not valid: %s", err.Error())
	}

	if len(removed) > 0 {
		removedIds, err := gcmd.LocatorsForDeploymentInstances(t, removed)
		if err != nil {
			gcmd.Fail(1, "Unable to generate deployment info: %s", err.Error())
		}

		failures := gcmd.Executor{
			On: removedIds,
			Serial: func(on gcmd.Locator) gcmd.JobRequest {
				return &cjobs.DeleteContainerRequest{
					Id: gcmd.AsIdentifier(on),
				}
			},
			Output: os.Stdout,
			OnSuccess: func(r *gcmd.CliJobResponse, w io.Writer, job gcmd.JobRequest) {
				fmt.Fprintf(w, "==> Deleted %s", string(job.(*cjobs.DeleteContainerRequest).Id))
			},
			Transport: t,
		}.Stream()
		for i := range failures {
			fmt.Fprintf(os.Stderr, failures[i].Error())
		}
	}

	addedIds, err := gcmd.LocatorsForDeploymentInstances(t, changes.Instances.Added())
	if err != nil {
		gcmd.Fail(1, "Unable to generate deployment info: %s", err.Error())
	}

	errors := gcmd.Executor{
		On: addedIds,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			instance, _ := changes.Instances.Find(gcmd.AsIdentifier(on))
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
		OnSuccess: func(r *gcmd.CliJobResponse, w io.Writer, job gcmd.JobRequest) {
			installJob := job.(*cjobs.InstallContainerRequest)
			instance, _ := changes.Instances.Find(installJob.Id)
			if pairs, ok := installJob.PortMappingsFrom(r.Pending); ok {
				if !instance.Ports.Update(pairs) {
					fmt.Fprintf(os.Stderr, "Not all ports listed %+v were returned by the server %+v", instance.Ports, pairs)
				}
			}
		},
		Output:    os.Stdout,
		Transport: t,
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

	contents, _ := json.MarshalIndent(changes, "", "  ")
	contents = append(contents, []byte("\n")...)
	if err := ioutil.WriteFile(newPath, contents, 0664); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to write %s: %s\n", newPath, err.Error())
	}

	linkedIds, err := gcmd.LocatorsForDeploymentInstances(t, changes.Instances.Linked())
	if err != nil {
		gcmd.Fail(1, "Unable to generate deployment info: %s", err.Error())
	}

	gcmd.Executor{
		On: linkedIds,
		Group: func(on ...gcmd.Locator) gcmd.JobRequest {
			links := []containers.ContainerLink{}
			for i := range on {
				instance, _ := changes.Instances.Find(gcmd.AsIdentifier(on[i]))
				network := instance.NetworkLinks()
				if len(network) > 0 {
					links = append(links, containers.ContainerLink{instance.Id, network})
				}
			}

			return &cjobs.LinkContainersRequest{&containers.ContainerLinks{links}}
		},
		Output:    os.Stdout,
		Transport: t,
	}.Stream()

	gcmd.Executor{
		On: addedIds,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			return &cjobs.StartedContainerStateRequest{
				Id: gcmd.AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		Transport: t,
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
		gcmd.Fail(1, err.Error())
	}

	if len(args) < 2 {
		gcmd.Fail(1, "Valid arguments: <image_name> <id> ...")
	}

	t := defaultTransport.Get()

	imageId := args[0]
	if imageId == "" {
		gcmd.Fail(1, "Argument 1 must be a Docker image to base the service on")
	}
	ids, err := gcmd.NewContainerLocators(t, args[1:]...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	for _, locator := range ids {
		if imageId == string(gcmd.AsIdentifier(locator)) {
			gcmd.Fail(1, "Image name and container id must not be the same: %s", imageId)
		}
	}

	gcmd.Executor{
		On: ids,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			r := cjobs.InstallContainerRequest{
				RequestIdentifier: jobs.NewRequestIdentifier(),

				Id:               gcmd.AsIdentifier(on),
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
		Transport: t,
	}.StreamAndExit()
}

func buildImage(cmd *cobra.Command, args []string) {
	if err := environment.ExtractVariablesFrom(&args, false); err != nil {
		gcmd.Fail(1, err.Error())
	}

	if len(args) < 3 {
		gcmd.Fail(1, "Valid arguments: <source> <build image> <tag> ...")
	}

	if buildReq.CallbackUrl != "" {
		_, err := url.ParseRequestURI(buildReq.CallbackUrl)
		if err != nil {
			gcmd.Fail(1, "The callbackUrl was an invalid URL")
		}
	}

	buildReq.Source = args[0]
	buildReq.BaseImage = args[1]
	buildReq.Tag = args[2]
	buildReq.Writer = os.Stdout
	buildReq.DockerSocket = conf.Docker.Socket
	buildReq.Environment = environment.Description.Map()

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
		gcmd.Fail(1, err.Error())
	}

	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <name>... <key>=<value>...")
	}

	t := defaultTransport.Get()

	ids, err := gcmd.NewContainerLocators(t, args...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	gcmd.Executor{
		On: ids,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			environment.Description.Id = gcmd.AsIdentifier(on)
			if resetEnv {
				return &cjobs.PutEnvironmentRequest{environment.Description}
			}

			return &cjobs.PatchEnvironmentRequest{environment.Description}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func showEnvironment(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <id> ...")
	}

	t := defaultTransport.Get()

	ids, err := gcmd.NewContainerLocators(t, args[0:]...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid environment ids: %s", err.Error())
	}

	data, errors := gcmd.Executor{
		On: ids,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			return &cjobs.ContentRequest{
				Locator: string(gcmd.AsIdentifier(on)),
				Type:    cjobs.ContentTypeEnvironment,
			}
		},
		Output:    os.Stdout,
		Transport: t,
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
	t := defaultTransport.Get()

	if err := gcmd.ExtractContainerLocatorsFromDeployment(t, deploymentPath, &args); err != nil {
		gcmd.Fail(1, err.Error())
	}

	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <id> ...")
	}

	ids, err := gcmd.NewContainerLocators(t, args...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	gcmd.Executor{
		On: ids,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			return &cjobs.DeleteContainerRequest{
				Id: gcmd.AsIdentifier(on),
			}
		},
		Output: os.Stdout,
		OnSuccess: func(r *gcmd.CliJobResponse, w io.Writer, job gcmd.JobRequest) {
			fmt.Fprintf(w, "Deleted %s", string(job.(*cjobs.DeleteContainerRequest).Id))
		},
		Transport: t,
	}.StreamAndExit()
}

func linkContainers(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <id> ...")
	}

	t := defaultTransport.Get()

	ids, err := gcmd.NewContainerLocators(t, args...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}
	if networkLinks.NetworkLinks == nil {
		networkLinks.NetworkLinks = &containers.NetworkLinks{}
	}

	gcmd.Executor{
		On: ids,
		Group: func(on ...gcmd.Locator) gcmd.JobRequest {
			links := &containers.ContainerLinks{make([]containers.ContainerLink, 0, len(on))}
			for i := range on {
				links.Links = append(links.Links, containers.ContainerLink{gcmd.AsIdentifier(on[i]), *networkLinks.NetworkLinks})
			}
			return &cjobs.LinkContainersRequest{links}
		},
		Output: os.Stdout,
		OnSuccess: func(r *gcmd.CliJobResponse, w io.Writer, job gcmd.JobRequest) {
			fmt.Fprintf(w, "Links set on %s\n", job.(*cjobs.LinkContainersRequest).ContainerLinks.String())
		},
		Transport: t,
	}.StreamAndExit()
}

func startContainer(cmd *cobra.Command, args []string) {
	t := defaultTransport.Get()

	if err := gcmd.ExtractContainerLocatorsFromDeployment(t, deploymentPath, &args); err != nil {
		gcmd.Fail(1, err.Error())
	}
	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := gcmd.NewContainerLocators(t, args...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	gcmd.Executor{
		On: ids,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			return &cjobs.StartedContainerStateRequest{
				Id: gcmd.AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func stopContainer(cmd *cobra.Command, args []string) {
	t := defaultTransport.Get()

	if err := gcmd.ExtractContainerLocatorsFromDeployment(t, deploymentPath, &args); err != nil {
		gcmd.Fail(1, err.Error())
	}
	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := gcmd.NewContainerLocators(t, args...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	gcmd.Executor{
		On: ids,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			return &cjobs.StoppedContainerStateRequest{
				Id: gcmd.AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func restartContainer(cmd *cobra.Command, args []string) {
	t := defaultTransport.Get()

	if err := gcmd.ExtractContainerLocatorsFromDeployment(t, deploymentPath, &args); err != nil {
		gcmd.Fail(1, err.Error())
	}
	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := gcmd.NewContainerLocators(t, args...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	gcmd.Executor{
		On: ids,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			return &cjobs.RestartContainerRequest{
				Id: gcmd.AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func containerStatus(cmd *cobra.Command, args []string) {
	t := defaultTransport.Get()

	if err := gcmd.ExtractContainerLocatorsFromDeployment(t, deploymentPath, &args); err != nil {
		gcmd.Fail(1, err.Error())
	}
	if len(args) < 1 {
		gcmd.Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := gcmd.NewContainerLocators(t, args...)
	if err != nil {
		gcmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	data, errors := gcmd.Executor{
		On: ids,
		Serial: func(on gcmd.Locator) gcmd.JobRequest {
			return &cjobs.ContainerStatusRequest{
				Id: gcmd.AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		Transport: t,
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
	t, servers := transportAndHosts(args...)

	data, errors := gcmd.Executor{
		On: servers,
		Group: func(on ...gcmd.Locator) gcmd.JobRequest {
			return &cjobs.ListContainersRequest{}
		},
		Output:    os.Stdout,
		Transport: t,
	}.Gather()

	combined := cjobs.ListServerContainersResponse{}
	for i := range data {
		// if r, ok := data[i].(*cjobs.ListServerContainersResponse); ok {
		// 	combined.Append(&r.ListContainersResponse)
		// } else
		if j, ok := data[i].(*cjobs.ListContainersResponse); ok {
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

func purge(cmd *cobra.Command, args []string) {
	t, servers := transportAndHosts(args...)

	gcmd.Executor{
		On: servers,
		Group: func(on ...gcmd.Locator) gcmd.JobRequest {
			return &cjobs.PurgeContainersRequest{}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

// func createToken(cmd *cobra.Command, args []string) {
// 	if len(args) != 2 {
// 		gcmd.Fail(1, "Valid arguments: <type> <content_id>")
// 	}

// 	if keyPath == "" {
// 		gcmd.Fail(1, "You must specify --key-path to create a token")
// 	}
// 	config, err := encrypted.NewTokenConfiguration(filepath.Join(keyPath, "client"), filepath.Join(keyPath, "server.pub"))
// 	if err != nil {
// 		gcmd.Fail(1, "Unable to load token configuration: %s", err.Error())
// 	}

// 	job := &cjobs.ContentRequest{Locator: args[1], Type: args[0]}
// 	value, err := config.Sign(job, "key", expiresAt)
// 	if err != nil {
// 		gcmd.Fail(1, "Unable to sign this request: %s", err.Error())
// 	}
// 	fmt.Printf("%s", value)
// 	os.Exit(0)
// }

func transportAndHosts(args ...string) (transport.Transport, gcmd.Locators) {
	t := defaultTransport.Get()

	if len(args) == 0 {
		args = []string{transport.Local.String()}
	}
	servers, err := gcmd.NewHostLocators(t, args[0:]...)
	if err != nil {
		gcmd.Fail(1, "You must pass zero or more valid host names (use '%s' or pass no arguments for the current server): %s", transport.Local.String(), err.Error())
	}

	return t, servers
}
