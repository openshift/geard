package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	cjobs "github.com/openshift/geard/containers/jobs"
	cloc "github.com/openshift/geard/containers/locator"
	"github.com/openshift/geard/deployment"
	"github.com/openshift/geard/encrypted"
	"github.com/openshift/geard/jobs"
	"github.com/openshift/geard/port"
	"github.com/openshift/geard/sti"
	"github.com/openshift/geard/transport"
	"github.com/spf13/cobra"
)

type CommandContext struct {
	Insecure  *bool
	Transport *transport.TransportFlag

	dockerSocket string

	resetEnv bool

	start        bool
	isolate      bool
	sockAct      bool
	systemdSlice string

	keyPath   string
	expiresAt int64

	environment  EnvironmentDescription
	portPairs    PortPairs
	networkLinks NetworkLinks

	deploymentPath string

	buildReq sti.STIRequest

	quiet   bool
	all     bool
	timeout int64
	noWait  bool
}

// Parse the command line arguments and invoke one of the support subcommands.
func (ctx *CommandContext) RegisterRemote(parent *cobra.Command) {
	parent.PersistentFlags().StringVar(&(ctx.deploymentPath), "with", "", "Provide a deployment descriptor to operate on")

	deployCmd := &cobra.Command{
		Use:   "deploy <file|url> <host>...",
		Short: "Deploy a set of containers to the named hosts",
		Long:  "Given a simple description of a group of containers, wire them together using the gear primitives.",
		Run:   ctx.deployContainers,
	}
	deployCmd.Flags().BoolVar(&(ctx.isolate), "isolate", false, "Use an isolated container running as a user")
	deployCmd.Flags().Int64VarP(&(ctx.timeout), "timeout", "", 300, "Number of seconds to wait for a response")
	parent.AddCommand(deployCmd)

	installImageCmd := &cobra.Command{
		Use:   "install <image> <name>... [<env>]",
		Short: "Install a docker image as a systemd service",
		Long:  "Install a docker image as one or more systemd services on one or more servers.\n\nSpecify a location on a remote server with <host>[:<port>]/<name> instead of <name>.  The default port is 2223.",
		Run:   ctx.installImage,
	}
	installImageCmd.Flags().VarP(&(ctx.portPairs), "ports", "p", "List of comma separated port pairs to bind '<internal>:<external>,...'. Use zero to request a port be assigned.")
	installImageCmd.Flags().VarP(&(ctx.networkLinks), "net-links", "n", "List of comma separated port pairs to wire '<local_host>:<local_port>:<remote_host>:<remote_port>,...'. local_host may be empty. It defaults to 127.0.0.1.")
	installImageCmd.Flags().BoolVar(&(ctx.start), "start", false, "Start the container immediately")
	installImageCmd.Flags().BoolVar(&(ctx.isolate), "isolate", false, "Use an isolated container running as a user")
	installImageCmd.Flags().BoolVar(&(ctx.sockAct), "socket-activated", false, "Use a socket-activated container (experimental, requires Docker branch)")
	installImageCmd.Flags().StringVar(&(ctx.environment.Path), "env-file", "", "Path to an environment file to load")
	installImageCmd.Flags().StringVar(&(ctx.environment.Description.Source), "env-url", "", "A url to download environment files from")
	installImageCmd.Flags().StringVar((*string)(&(ctx.environment.Description.Id)), "env-id", "", "An optional identifier for the environment being set")
	installImageCmd.Flags().StringVar(&(ctx.systemdSlice), "slice", cjobs.DefaultSlice, "systemd slice to use. default: "+cjobs.DefaultSlice)
	parent.AddCommand(installImageCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <name>...",
		Short: "Delete an installed container",
		Long:  "Deletes one or more installed containers from the system.  Will not clean up unused images.",
		Run:   ctx.deleteContainer,
	}
	parent.AddCommand(deleteCmd)

	buildCmd := &cobra.Command{
		Use:   "build <source> <image> <tag> [<env>]",
		Short: "(Local) Build a new image on this host",
		Long:  "Build a new Docker image named <tag> from a source repository and base image.",
		Run:   ctx.buildImage,
	}
	buildCmd.Flags().StringVarP(&(ctx.dockerSocket), "docker-socket", "S", "unix:///var/run/docker.sock", "Set the docker socket to use")
	buildCmd.Flags().BoolVar(&(ctx.buildReq.Clean), "clean", false, "Perform a clean build")
	buildCmd.Flags().StringVarP(&(ctx.buildReq.Ref), "ref", "r", "", "Specify a ref to check-out")
	buildCmd.Flags().BoolVar(&(ctx.buildReq.Verbose), "verbose", false, "Enable verbose output")
	buildCmd.Flags().StringVar(&(ctx.buildReq.CallbackUrl), "callbackUrl", "", "Specify a URL to invoke via HTTP POST upon build completion")
	buildCmd.Flags().StringVar(&(ctx.environment.Path), "env-file", "", "Path to an environment file to load")
	buildCmd.Flags().StringVar(&(ctx.environment.Description.Source), "env-url", "", "A url to download environment files from")
	buildCmd.Flags().StringVarP(&(ctx.buildReq.ScriptsUrl), "scripts", "s", "", "Specify a URL for the assemble and run scripts")
	buildCmd.Flags().BoolVar(&(ctx.buildReq.RemovePreviousImage), "rm", false, "Remove the previous image during incremental builds")

	parent.AddCommand(buildCmd)

	setEnvCmd := &cobra.Command{
		Use:   "set-env <name>... [<env>]",
		Short: "Set environment variable values on servers",
		Long:  "Adds the listed environment values to the specified locations. The name is the environment id that multiple containers may reference. You can pass an environment file or key value pairs on the commandline.",
		Run:   ctx.setEnvironment,
	}
	setEnvCmd.Flags().BoolVar(&(ctx.resetEnv), "reset", false, "Remove any existing values")
	setEnvCmd.Flags().StringVar(&(ctx.environment.Path), "env-file", "", "Path to an environment file to load")
	parent.AddCommand(setEnvCmd)

	envCmd := &cobra.Command{
		Use:   "env <name>...",
		Short: "Retrieve environment variable values by id",
		Long:  "Return the environment variables matching the provided ids",
		Run:   ctx.showEnvironment,
	}
	parent.AddCommand(envCmd)

	linkCmd := &cobra.Command{
		Use:   "link <name>...",
		Short: "Set network links for the named containers",
		Long:  "Sets the network links for the named containers. A restart may be required to use the latest links.",
		Run:   ctx.linkContainers,
	}
	linkCmd.Flags().VarP(&(ctx.networkLinks), "net-links", "n", "List of comma separated port pairs to wire '<local_host>:local_port>:<host>:<remote_port>,...'. local_host may be empty. It defaults to 127.0.0.1")
	parent.AddCommand(linkCmd)

	startCmd := &cobra.Command{
		Use:   "start <name>...",
		Short: "Invoke systemd to start a container",
		Long:  "Queues the start and immediately returns.", //  Use -f to attach to the logs.",
		Run:   ctx.startContainer,
	}
	parent.AddCommand(startCmd)

	stopCmd := &cobra.Command{
		Use:   "stop <name>... [--no-wait]",
		Short: "Invoke systemd to stop a container",
		Long:  "Stop the specified container.  Waits for the container to stop unless --no-wait is specified",
		Run:   ctx.stopContainer,
	}
	stopCmd.Flags().BoolVarP(&(ctx.noWait), "no-wait", "n", false, "Do not wait for the container to stop")
	parent.AddCommand(stopCmd)

	restartCmd := &cobra.Command{
		Use:   "restart <name>...",
		Short: "Invoke systemd to restart a container",
		Long:  "Queues the restart and immediately returns.", //  Use -f to attach to the logs.",
		Run:   ctx.restartContainer,
	}
	parent.AddCommand(restartCmd)

	statusCmd := &cobra.Command{
		Use:   "status <name>...",
		Short: "Retrieve the systemd status of one or more containers",
		Long:  "Shows the equivalent of 'systemctl status ctr-<name>' for each listed unit",
		Run:   ctx.containerStatus,
	}
	parent.AddCommand(statusCmd)

	listUnitsCmd := &cobra.Command{
		Use:   "list-units <host>...",
		Short: "Retrieve the list of services across all hosts",
		Long:  "Shows the equivalent of 'systemctl list-units ctr-<name>' for each installed container",
		Run:   ctx.listUnits,
	}
	listUnitsCmd.Flags().BoolVarP(&(ctx.all), "all", "a", false, "Include inactive or unloaded containers")
	listUnitsCmd.Flags().BoolVarP(&(ctx.quiet), "quiet", "q", false, "Return only the id of each unit")
	parent.AddCommand(listUnitsCmd)

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Stop and disable all containers",
		Long:  "Disable all registered resources from systemd to allow them to be removed from the system.  Will reload the systemd daemon config.",
		Run:   ctx.purge,
	}
	parent.AddCommand(purgeCmd)
}

func (ctx *CommandContext) RegisterLocal(parent *cobra.Command) {
	createTokenCmd := &cobra.Command{
		Use:   "create-token <type> <content_id>",
		Short: "(Local) Generate a content request token",
		Long:  "Create a URL that will serve as a content request token using a server public key and client private key.",
		Run:   ctx.createToken,
	}
	createTokenCmd.Flags().StringVar(&(ctx.keyPath), "key-path", "", "Specify the directory containing the client private and server public keys")
	createTokenCmd.Flags().Int64Var(&(ctx.expiresAt), "expires-at", time.Now().Unix()+3600, "Specify the content request token expiration time in seconds after the Unix epoch")
	parent.AddCommand(createTokenCmd)
}

func (ctx *CommandContext) deployContainers(c *cobra.Command, args []string) {
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <deployment_file|URL> <host> ...")
	}

	t := ctx.Transport.Get()

	path := args[0]
	if path == "" {
		cmd.Fail(1, "Argument 1 must be deployment file or URL describing how the containers are related")
	}

	u, err := url.Parse(path)
	if nil != err {
		cmd.Fail(1, "Cannot Parse Argument 1: %s", err.Error())
	}

	var deploy *deployment.Deployment
	switch u.Scheme {
	case "":
		deploy, err = deployment.NewDeploymentFromFile(u.Path)
	case "file":
		deploy, err = deployment.NewDeploymentFromFile(u.Path)
	case "http", "https":
		deploy, err = deployment.NewDeploymentFromURL(u.String(), *ctx.Insecure, time.Duration(ctx.timeout))
	default:
		cmd.Fail(1, "Unsupported URL Scheme '%s' for deployment", u.Scheme)
	}

	if nil != err {
		cmd.Fail(1, "Unable to load deployment from %s: %s", path, err.Error())
	}

	if len(args) == 1 {
		args = append(args, transport.Local.String())
	}
	servers, err := transport.NewTransportLocators(t, args[1:]...)
	if err != nil {
		cmd.Fail(1, "You must pass zero or more valid host names (use '%s' or pass no arguments for the current server): %s", transport.Local.String(), err.Error())
	}

	re := regexp.MustCompile("\\.\\d{8}\\-\\d{6}\\z")
	now := time.Now().Format(".20060102-150405")
	base := filepath.Base(path)
	base = re.ReplaceAllString(base, "")
	newPath := base + now

	fmt.Printf("==> Deploying %s\n", path)
	changes, removed, err := deploy.Describe(deployment.SimplePlacement(servers), t)
	if err != nil {
		cmd.Fail(1, "Deployment is not valid: %s", err.Error())
	}

	if len(removed) > 0 {
		removedIds, err := LocatorsForDeploymentInstances(t, removed)
		if err != nil {
			cmd.Fail(1, "Unable to generate deployment info: %s", err.Error())
		}

		failures := cmd.Executor{
			On: removedIds,
			Serial: func(on cmd.Locator) cmd.JobRequest {
				return &cjobs.DeleteContainerRequest{
					Id: cloc.AsIdentifier(on),
				}
			},
			Output: os.Stdout,
			OnSuccess: func(r *cmd.CliJobResponse, w io.Writer, job cmd.RequestedJob) {
				fmt.Fprintf(w, "==> Deleted %s", string(job.Request.(*cjobs.DeleteContainerRequest).Id))
			},
			Transport: t,
		}.Stream()
		for i := range failures {
			fmt.Fprintf(os.Stderr, failures[i].Error())
		}
	}

	addedIds, err := LocatorsForDeploymentInstances(t, changes.Instances.Added())
	if err != nil {
		cmd.Fail(1, "Unable to generate deployment info: %s", err.Error())
	}

	errors := cmd.Executor{
		On: addedIds,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			instance, _ := changes.Instances.Find(cloc.AsIdentifier(on))
			links := instance.NetworkLinks()

			return &cjobs.InstallContainerRequest{
				RequestIdentifier: jobs.NewRequestIdentifier(),

				Id:          instance.Id,
				Image:       instance.Image,
				Environment: instance.EnvironmentVariables(),
				Isolate:     ctx.isolate,

				Ports:        instance.Ports.PortPairs(),
				NetworkLinks: &links,
			}
		},
		OnSuccess: func(r *cmd.CliJobResponse, w io.Writer, job cmd.RequestedJob) {
			installJob := job.Request.(*cjobs.InstallContainerRequest)
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

	linkedIds, err := LocatorsForDeploymentInstances(t, changes.Instances.Linked())
	if err != nil {
		cmd.Fail(1, "Unable to generate deployment info: %s", err.Error())
	}

	cmd.Executor{
		On: linkedIds,
		Group: func(on ...cmd.Locator) cmd.JobRequest {
			links := []containers.ContainerLink{}
			for i := range on {
				instance, _ := changes.Instances.Find(cloc.AsIdentifier(on[i]))
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

	cmd.Executor{
		On: addedIds,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			return &cjobs.StartedContainerStateRequest{
				Id: cloc.AsIdentifier(on),
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

func (ctx *CommandContext) installImage(c *cobra.Command, args []string) {
	if err := ctx.environment.ExtractVariablesFrom(&args, true); err != nil {
		cmd.Fail(1, err.Error())
	}

	if len(args) < 2 {
		cmd.Fail(1, "Valid arguments: <image_name> <id> ...")
	}

	t := ctx.Transport.Get()

	imageId := args[0]
	if imageId == "" {
		cmd.Fail(1, "Argument 1 must be a Docker image to base the service on")
	}
	ids, err := cloc.NewContainerLocators(t, args[1:]...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	for _, locator := range ids {
		if imageId == string(cloc.AsIdentifier(locator)) {
			cmd.Fail(1, "Image name and container id must not be the same: %s", imageId)
		}
	}

	cmd.Executor{
		On: ids,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			r := cjobs.InstallContainerRequest{
				RequestIdentifier: jobs.NewRequestIdentifier(),

				Id:               cloc.AsIdentifier(on),
				Image:            imageId,
				Started:          ctx.start,
				Isolate:          ctx.isolate,
				SocketActivation: ctx.sockAct,

				Ports:        *ctx.portPairs.Get().(*port.PortPairs),
				Environment:  &ctx.environment.Description,
				NetworkLinks: ctx.networkLinks.NetworkLinks,
				SystemdSlice: ctx.systemdSlice,
			}
			return &r
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func (ctx *CommandContext) buildImage(c *cobra.Command, args []string) {
	if err := ctx.environment.ExtractVariablesFrom(&args, false); err != nil {
		cmd.Fail(1, err.Error())
	}

	if len(args) < 3 {
		cmd.Fail(1, "Valid arguments: <source> <image> <tag> ...")
	}

	buildReq := &ctx.buildReq

	if buildReq.CallbackUrl != "" {
		_, err := url.ParseRequestURI(buildReq.CallbackUrl)
		if err != nil {
			cmd.Fail(1, "The callbackUrl was an invalid URL")
		}
	}

	buildReq.Source = args[0]
	buildReq.BaseImage = args[1]
	buildReq.Tag = args[2]
	buildReq.Writer = os.Stdout
	buildReq.DockerSocket = ctx.dockerSocket
	buildReq.Environment = ctx.environment.Description.Map()

	res, err := sti.Build(buildReq)
	if err != nil {
		fmt.Printf("An error occured: %s\n", err.Error())
		return
	}

	for _, message := range res.Messages {
		fmt.Println(message)
	}
}

func (ctx *CommandContext) setEnvironment(c *cobra.Command, args []string) {
	if err := ctx.environment.ExtractVariablesFrom(&args, false); err != nil {
		cmd.Fail(1, err.Error())
	}

	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <name>... <key>=<value>...")
	}

	t := ctx.Transport.Get()

	ids, err := cloc.NewContainerLocators(t, args...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	cmd.Executor{
		On: ids,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			ctx.environment.Description.Id = cloc.AsIdentifier(on)
			if ctx.resetEnv {
				return &cjobs.PutEnvironmentRequest{ctx.environment.Description}
			}

			return &cjobs.PatchEnvironmentRequest{ctx.environment.Description}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func (ctx *CommandContext) showEnvironment(c *cobra.Command, args []string) {
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> ...")
	}

	t := ctx.Transport.Get()

	ids, err := cloc.NewContainerLocators(t, args[0:]...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid environment ids: %s", err.Error())
	}

	data, errors := cmd.Executor{
		On: ids,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			return &cjobs.GetEnvironmentRequest{
				Id: cloc.AsIdentifier(on),
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

func (ctx *CommandContext) deleteContainer(c *cobra.Command, args []string) {
	t := ctx.Transport.Get()

	if err := ExtractContainerLocatorsFromDeployment(t, ctx.deploymentPath, &args); err != nil {
		cmd.Fail(1, err.Error())
	}

	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> ...")
	}

	ids, err := cloc.NewContainerLocators(t, args...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	cmd.Executor{
		On: ids,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			return &cjobs.DeleteContainerRequest{
				Id: cloc.AsIdentifier(on),
			}
		},
		Output: os.Stdout,
		OnSuccess: func(r *cmd.CliJobResponse, w io.Writer, job cmd.RequestedJob) {
			fmt.Fprintf(w, "Deleted %s", string(job.Request.(*cjobs.DeleteContainerRequest).Id))
		},
		Transport: t,
	}.StreamAndExit()
}

func (ctx *CommandContext) linkContainers(c *cobra.Command, args []string) {
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> ...")
	}

	t := ctx.Transport.Get()

	ids, err := cloc.NewContainerLocators(t, args...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}
	if ctx.networkLinks.NetworkLinks == nil {
		ctx.networkLinks.NetworkLinks = &containers.NetworkLinks{}
	}

	cmd.Executor{
		On: ids,
		Group: func(on ...cmd.Locator) cmd.JobRequest {
			links := &containers.ContainerLinks{make([]containers.ContainerLink, 0, len(on))}
			for i := range on {
				links.Links = append(links.Links, containers.ContainerLink{cloc.AsIdentifier(on[i]), *ctx.networkLinks.NetworkLinks})
			}
			return &cjobs.LinkContainersRequest{links}
		},
		Output: os.Stdout,
		OnSuccess: func(r *cmd.CliJobResponse, w io.Writer, job cmd.RequestedJob) {
			fmt.Fprintf(w, "Links set on %s\n", job.Request.(*cjobs.LinkContainersRequest).ContainerLinks.String())
		},
		Transport: t,
	}.StreamAndExit()
}

func (ctx *CommandContext) startContainer(c *cobra.Command, args []string) {
	t := ctx.Transport.Get()

	if err := ExtractContainerLocatorsFromDeployment(t, ctx.deploymentPath, &args); err != nil {
		cmd.Fail(1, err.Error())
	}
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := cloc.NewContainerLocators(t, args...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	cmd.Executor{
		On: ids,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			return &cjobs.StartedContainerStateRequest{
				Id: cloc.AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func (ctx *CommandContext) stopContainer(c *cobra.Command, args []string) {
	t := ctx.Transport.Get()

	if err := ExtractContainerLocatorsFromDeployment(t, ctx.deploymentPath, &args); err != nil {
		cmd.Fail(1, err.Error())
	}
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := cloc.NewContainerLocators(t, args...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	cmd.Executor{
		On: ids,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			return &cjobs.StoppedContainerStateRequest{
				Id:   cloc.AsIdentifier(on),
				Wait: !ctx.noWait,
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func (ctx *CommandContext) restartContainer(c *cobra.Command, args []string) {
	t := ctx.Transport.Get()

	if err := ExtractContainerLocatorsFromDeployment(t, ctx.deploymentPath, &args); err != nil {
		cmd.Fail(1, err.Error())
	}
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := cloc.NewContainerLocators(t, args...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	cmd.Executor{
		On: ids,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			return &cjobs.RestartContainerRequest{
				Id: cloc.AsIdentifier(on),
			}
		},
		Output:    os.Stdout,
		Transport: t,
	}.StreamAndExit()
}

func (ctx *CommandContext) containerStatus(c *cobra.Command, args []string) {
	t := ctx.Transport.Get()

	if err := ExtractContainerLocatorsFromDeployment(t, ctx.deploymentPath, &args); err != nil {
		cmd.Fail(1, err.Error())
	}
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> ...")
	}
	ids, err := cloc.NewContainerLocators(t, args...)
	if err != nil {
		cmd.Fail(1, "You must pass one or more valid service names: %s", err.Error())
	}

	data, errors := cmd.Executor{
		On: ids,
		Serial: func(on cmd.Locator) cmd.JobRequest {
			return &cjobs.ContainerStatusRequest{
				Id: cloc.AsIdentifier(on),
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

func (ctx *CommandContext) listUnits(c *cobra.Command, args []string) {
	t, servers := ctx.transportAndHosts(args...)

	data, errors := cmd.Executor{
		On: servers,
		Group: func(on ...cmd.Locator) cmd.JobRequest {
			return &cjobs.ListContainersRequest{IncludeInactive: ctx.all}
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
	if ctx.quiet {
		for i := range combined.Containers {
			c := &combined.Containers[i]
			if c.Server != "" {
				fmt.Fprintf(os.Stdout, "%s/%s\n", c.Server, c.Id)
			} else {
				fmt.Fprintf(os.Stdout, "%s\n", c.Id)
			}
		}
	} else {
		combined.WriteTableTo(os.Stdout)
	}
	if len(errors) > 0 {
		for i := range errors {
			fmt.Fprintf(os.Stderr, "Error: %s\n", errors[i])
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func (ctx *CommandContext) purge(c *cobra.Command, args []string) {
	t, servers := ctx.transportAndHosts(args...)

	cmd.Executor{
		On: servers,
		Group: func(on ...cmd.Locator) cmd.JobRequest {
			return &cjobs.PurgeContainersRequest{}
		},
		Output: os.Stdout,
		OnSuccess: func(r *cmd.CliJobResponse, w io.Writer, job cmd.RequestedJob) {
			fmt.Fprintf(w, "Stopped and removed all containers from %s", job.Locator.TransportLocator().String())
		},
		Transport: t,
	}.StreamAndExit()
}

func (ctx *CommandContext) createToken(c *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Fail(1, "Valid arguments: <content>")
	}

	if ctx.keyPath == "" {
		cmd.Fail(1, "You must specify --key-path to create a token")
	}
	config, err := encrypted.NewTokenConfiguration(filepath.Join(ctx.keyPath, "client"), filepath.Join(ctx.keyPath, "server.pub"))
	if err != nil {
		cmd.Fail(1, "Unable to load token configuration: %s", err.Error())
	}

	value, err := config.Sign(args[0], "key", ctx.expiresAt)
	if err != nil {
		cmd.Fail(1, "Unable to sign this request: %s", err.Error())
	}
	fmt.Printf("%s", value)
	os.Exit(0)
}

func (ctx *CommandContext) transportAndHosts(args ...string) (transport.Transport, cmd.Locators) {
	t := ctx.Transport.Get()

	if len(args) == 0 {
		args = []string{transport.Local.String()}
	}
	servers, err := cmd.NewHostLocators(t, args[0:]...)
	if err != nil {
		cmd.Fail(1, "You must pass zero or more valid host names (use '%s' or pass no arguments for the current server): %s", transport.Local.String(), err.Error())
	}

	return t, servers
}
