package cmd

import (
	"bufio"
	"bytes"
	"code.google.com/p/go.crypto/ssh"
	"fmt"
	"github.com/smarterclayton/cobra"
	"github.com/smarterclayton/geard/containers"
	"github.com/smarterclayton/geard/dispatcher"
	"github.com/smarterclayton/geard/encrypted"
	"github.com/smarterclayton/geard/git"
	githttp "github.com/smarterclayton/geard/git/http"
	gitjobs "github.com/smarterclayton/geard/git/jobs"
	"github.com/smarterclayton/geard/http"
	"github.com/smarterclayton/geard/jobs"
	"github.com/smarterclayton/geard/systemd"
	"io"
	"io/ioutil"
	"log"
	nethttp "net/http"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	//	"crypto/sha256"
)

var (
	pre          bool
	post         bool
	follow       bool
	start        bool
	listenAddr   string
	resetEnv     bool
	simple       bool
	keyPath      string
	environment  EnvironmentDescription
	portPairs    PortPairs
	networkLinks NetworkLinks
	gitKeys      bool
	gitRepoName  string
	gitRepoURL   string
	keyFile      string
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
		Long:  "Shows the equivalent of 'systemctl status container-<name>' for each listed unit",
		Run:   containerStatus,
	}
	gearCmd.AddCommand(statusCmd)

	listUnitsCmd := &cobra.Command{
		Use:   "list-units <host>...",
		Short: "Retrieve the list of services across all hosts",
		Long:  "Shows the equivalent of 'systemctl list-units container-<name>' for each installed container",
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
		Short: "(Local) Generate authoried keys output for sshd.",
		Long:  "Generate authoried keys output for sshd. See sshd_config(5)#AuthorizedKeysCommand",
		Run:   sshAuthKeysCommand,
	}
	gearCmd.AddCommand(sshAuthKeysCmd)

	sshKeysCmd := &cobra.Command{
		Use:   "keys",
		Short: "Add a public key to enable SSH access to a repository or container location",
		Long:  "Add a public key to enable SSH access to a repository or container location.",
		Run:   sshKeysAdd,
	}
	sshKeysCmd.Flags().StringVar(&keyFile, "key-file", "", "read input from FILE specified matching sshd AuthorizedKeysFile format")
	gearCmd.AddCommand(sshKeysCmd)

	if err := gearCmd.Execute(); err != nil {
		fail(1, err.Error())
	}
}

func ExecuteSshAuthKeysCmd(args ...string) {
	if len(args) != 2 {
		os.Exit(2)
	}
	SshAuthKeysCommand(nil, args[1:])
}

func SshAuthKeysCommand(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		Fail(1, "Valid arguments: <login name>\n")
	}

	var (
		u           *user.User
		err         error
		containerId containers.Identifier
		repoId      git.RepoIdentifier
	)

	if u, err = user.Lookup(args[0]); err != nil {
		Fail(2, "Unable to lookup user")
	}

	isRepo := u.Name == "Repository user"
	if isRepo {
		repoId, err = git.NewIdentifierFromUser(u)
		if err != nil {
			Fail(1, "Not a repo user: %s\n", err.Error())
		}
	} else {
		containerId, err = containers.NewIdentifierFromUser(u)
		if err != nil {
			Fail(1, "Not a container user: %s\n", err.Error())
		}
	}

	if isRepo {
		if err := git.GenerateAuthorizedKeys(repoId, u, false, true); err != nil {
			Fail(2, "Unable to generate authorized_keys file: %s\n", err.Error())
		}
	} else {
		if err := containers.GenerateAuthorizedKeys(containerId, u, false, true); err != nil {
			Fail(2, "Unable to generate authorized_keys file: %s\n", err.Error())
		}
	}
}

func sshKeysAdd(cmd *cobra.Command, args []string) {

	var (
		data  []byte
		keys  []jobs.KeyData
		err   error
		write bool
	)

	// default to false for write
	write = false

	// validate that arguments for locators are passsed
	if len(args) < 1 {
		fail(1, "Valid arguments: [LOCATOR] ...\n")
	}
	// args... are locators for repositories or containers
	ids, err := NewRemoteIdentifiers(args...)
	if err != nil {
		fail(1, "You must pass 1 or more valid LOCATOR names: %s\n", err.Error())
	}

	// keyFile - contains the sshd AuthorizedKeysFile location
	// Stdin - contains the AuthorizedKeysFile if keyFile is not specified
	if len(keyFile) != 0 {
		absPath, _ := filepath.Abs(keyFile)
		data, err = ioutil.ReadFile(absPath)
		if err != nil {
			fail(1, "You must pass a valid FILE that exists.\n%v", err.Error())
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
			fail(1, "Unable to parse authorized key from input")
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
					r = append(r, jobs.RepositoryPermission{cId, write})
				}
			}

			fmt.Println("Invoking Create keys Request")
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
