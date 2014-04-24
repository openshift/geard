package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	. "github.com/openshift/geard/cmd"
	"github.com/openshift/geard/containers"
	"github.com/openshift/geard/jobs"
	sshkey "github.com/openshift/geard/pkg/ssh-public-key"
	"github.com/openshift/geard/ssh"
	. "github.com/openshift/geard/ssh/jobs"
	. "github.com/openshift/geard/transport"
)

var (
	keyFile string
	handler serializeContainerPermission
)

// Implements the default container permission serialization
type serializeContainerPermission struct{}

func (c *serializeContainerPermission) CreatePermission(cmd *cobra.Command, id string) (*KeyPermission, error) {
	return NewKeyPermission(ssh.ContainerPermissionType, id)
}
func (c *serializeContainerPermission) DefineFlags(cmd *cobra.Command) {
}

func init() {
	AddPermissionCommand(ResourceTypeContainer, &handler)
}

func registerLocal(parent *cobra.Command) {
	keysForUserCmd := &cobra.Command{
		Use:   "auth-keys-command <username>",
		Short: "(Local) Generate authorized_keys output for sshd.",
		Long:  "Generate authorized keys output for sshd. See sshd_config(5)#AuthorizedKeysCommand",
		Run:   KeysForUser,
	}
	parent.AddCommand(keysForUserCmd)
}

func KeysForUser(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		Fail(1, "Valid arguments: <login name>\n")
	}

	u, err := user.Lookup(args[0])
	if err != nil {
		Fail(2, "Unable to lookup user")
	}

	if err := ssh.GenerateAuthorizedKeysFor(u, false, false); err != nil {
		Fail(1, "Unable to generate authorized_keys file: %s", err.Error())
	}
}

func registerRemote(parent *cobra.Command) {
	addKeysCmd := &cobra.Command{
		Use:   "add-keys <id>...",
		Short: "Set keys for SSH access to a resource",
		Long:  "Upload the provided public keys and enable SSH access to the specified resources.",
		Run:   addSshKeys,
	}
	addKeysCmd.Flags().StringVar(&keyFile, "key-file", "", "read input from file specified matching sshd AuthorizedKeysFile format")
	defineFlags(addKeysCmd)
	parent.AddCommand(addKeysCmd)
}

func addSshKeys(cmd *cobra.Command, args []string) {
	// validate that arguments for locators are passsed
	if len(args) < 1 {
		Fail(1, "Valid arguments: <id> ...")
	}
	// args... are locators for repositories or containers
	ids, err := NewGenericLocators(ResourceTypeContainer, args...)
	if err != nil {
		Fail(1, "You must pass 1 or more valid names: %s", err.Error())
	}

	transportName := cmd.Flags().Lookup("transport").Value.String()
	transport := GetTransport(transportName)
	if transport == nil {
		Fail(1, "Invalid transport: %s. Choices are: %v\n", transportName, GetTransportNames())
	}

	keys, err := readAuthorizedKeysFile(keyFile)
	if err != nil {
		Fail(1, "Unable to read authorized keys file: %s", err.Error())
	}

	allPerms := make(map[string]*KeyPermission)
	for i := range ids {
		if permissionHandlers == nil {
			Fail(1, "The type '%s' is not supported by this command", ids[i].ResourceType())
		}
		h, ok := permissionHandlers[ids[i].ResourceType()]
		if !ok {
			Fail(1, "The type '%s' is not supported by this command", ids[i].ResourceType())
		}
		perm, err := h.CreatePermission(cmd, string(ids[i].(ResourceLocator).Identifier()))
		if err != nil {
			Fail(1, err.Error())
		}
		allPerms[ids[i].Identity()] = perm
	}

	Executor{
		On: ids,
		Group: func(on ...Locator) jobs.Job {
			permissions := []KeyPermission{}
			for i := range on {
				permissions = append(permissions, *allPerms[on[i].Identity()])
			}

			r := CreateKeysRequest{
				&ExtendedCreateKeysData{
					Keys:        keys,
					Permissions: permissions,
				},
			}

			return transport.RequestFor(&r)

		},
		Output: os.Stdout,
		//TODO: display partial error info
		LocalInit: containers.InitializeData,
		Transport: transport,
	}.StreamAndExit()
}

func readAuthorizedKeysFile(keyFile string) ([]KeyData, error) {
	var (
		data []byte
		keys []KeyData
		err  error
	)

	// keyFile - contains the sshd AuthorizedKeysFile location
	// Stdin - contains the AuthorizedKeysFile if keyFile is not specified
	if len(keyFile) != 0 {
		absPath, _ := filepath.Abs(keyFile)
		data, err = ioutil.ReadFile(absPath)
		if err != nil {
			return keys, err
		}
	} else {
		data, _ = ioutil.ReadAll(os.Stdin)
	}

	bytesReader := bytes.NewReader(data)
	scanner := bufio.NewScanner(bytesReader)
	for scanner.Scan() {
		// Parse the AuthorizedKeys line
		pk, _, _, _, ok := sshkey.ParseAuthorizedKey(scanner.Bytes())
		if !ok {
			return keys, errors.New("Unable to parse authorized key from input source, invalid format")
		}
		value := sshkey.MarshalAuthorizedKey(pk)
		key, err := NewKeyData("authorized_keys", string(value))
		if err != nil {
			return keys, err
		}
		keys = append(keys, *key)
	}

	return keys, nil
}
