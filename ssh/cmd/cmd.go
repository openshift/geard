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

	"github.com/openshift/geard/cmd"
	cloc "github.com/openshift/geard/containers/locator"
	sshkey "github.com/openshift/geard/pkg/ssh-public-key"
	"github.com/openshift/geard/ssh"
	"github.com/openshift/geard/ssh/jobs"
	"github.com/openshift/geard/transport"
)

// Implements the default container permission serialization
type serializeContainerPermission struct{}

func (p *serializeContainerPermission) CreatePermission(c *cobra.Command, id string) (*jobs.KeyPermission, error) {
	return jobs.NewKeyPermission(ssh.ContainerPermissionType, id)
}
func (p *serializeContainerPermission) DefineFlags(c *cobra.Command) {
}

func init() {
	AddPermissionCommand(cloc.ResourceTypeContainer, &serializeContainerPermission{})
}

func RegisterAuthorizedKeys(parent *cobra.Command) {
	keysForUserCmd := &cobra.Command{
		Use:   "auth-keys-command <username>",
		Short: "(Local) Generate authorized_keys output for sshd",
		Long:  "Generate authorized keys output for sshd. See sshd_config(5)#AuthorizedKeysCommand",
		Run:   keysForUser,
	}
	parent.AddCommand(keysForUserCmd)
}

func keysForUser(c *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Fail(1, "Valid arguments: <login name>\n")
	}

	u, err := user.Lookup(args[0])
	if err != nil {
		cmd.Fail(2, "Unable to lookup user")
	}

	if err := ssh.GenerateAuthorizedKeysFor(u, false, false); err != nil {
		cmd.Fail(1, "Unable to generate authorized_keys file: %s", err.Error())
	}
}

type CommandContext struct {
	Transport *transport.TransportFlag

	keyFile string
}

func (ctx *CommandContext) RegisterAddKeys(parent *cobra.Command) {
	addKeysCmd := &cobra.Command{
		Use:   "add-keys <id>...",
		Short: "Set keys for SSH access to a resource",
		Long:  "Upload the provided public keys and enable SSH access to the specified resources.",
		Run:   ctx.addSshKeys,
	}
	addKeysCmd.Flags().StringVar(&ctx.keyFile, "key-file", "", "read input from file specified matching sshd AuthorizedKeysFile format")
	defineFlags(addKeysCmd)
	parent.AddCommand(addKeysCmd)
}

func (ctx *CommandContext) addSshKeys(c *cobra.Command, args []string) {
	// validate that arguments for locators are passsed
	if len(args) < 1 {
		cmd.Fail(1, "Valid arguments: <id> ...")
	}

	t := ctx.Transport.Get()

	// args... are locators for repositories or containers
	ids, err := cmd.NewResourceLocators(t, cloc.ResourceTypeContainer, args...)
	if err != nil {
		cmd.Fail(1, "You must pass 1 or more valid names: %s", err.Error())
	}

	keys, err := readAuthorizedKeysFile(ctx.keyFile)
	if err != nil {
		cmd.Fail(1, "Unable to read authorized keys file: %s", err.Error())
	}

	allPerms := make(map[string]*jobs.KeyPermission)
	for i := range ids {
		resourceType := ids[i].(*cmd.ResourceLocator).Type
		if permissionHandlers == nil {
			cmd.Fail(1, "The type '%s' is not supported by this command", resourceType)
		}
		h, ok := permissionHandlers[resourceType]
		if !ok {
			cmd.Fail(1, "The type '%s' is not supported by this command", resourceType)
		}
		perm, err := h.CreatePermission(c, ids[i].(*cmd.ResourceLocator).Id)
		if err != nil {
			cmd.Fail(1, err.Error())
		}
		allPerms[ids[i].Identity()] = perm
	}

	cmd.Executor{
		On: ids,
		Group: func(on ...cmd.Locator) cmd.JobRequest {
			permissions := []jobs.KeyPermission{}
			for i := range on {
				permissions = append(permissions, *allPerms[on[i].Identity()])
			}

			return &jobs.CreateKeysRequest{
				&jobs.ExtendedCreateKeysData{
					Keys:        keys,
					Permissions: permissions,
				},
			}
		},
		Output: os.Stdout,
		//TODO: display partial error info
		Transport: t,
	}.StreamAndExit()
}

func readAuthorizedKeysFile(keyFile string) ([]jobs.KeyData, error) {
	var (
		data []byte
		keys []jobs.KeyData
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
		key, err := jobs.NewKeyData("authorized_keys", string(value))
		if err != nil {
			return keys, err
		}
		keys = append(keys, *key)
	}

	return keys, nil
}
