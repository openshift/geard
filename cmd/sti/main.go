package main

import (
	"errors"
	"fmt"
	"log"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/openshift/geard/sti"
	"github.com/spf13/cobra"
)

var version string

func parseEnvs(envStr string) (map[string]string, error) {
	if envStr == "" {
		return nil, nil
	}

	envs := make(map[string]string)
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

func Execute() {
	var (
		req       *sti.STIRequest
		envString string
	)

	req = &sti.STIRequest{}

	stiCmd := &cobra.Command{
		Use:   "sti",
		Short: "STI is a tool for building repeatable docker images",
		Long: `A command-line interface for the sti library
              Complete documentation is available at http://github.com/pmorie/go-sti`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}
	stiCmd.PersistentFlags().StringVarP(&(req.DockerSocket), "url", "U", "unix:///var/run/docker.sock", "Set the url of the docker socket to use")
	stiCmd.PersistentFlags().BoolVar(&(req.Verbose), "verbose", false, "Enable verbose output")
	stiCmd.PersistentFlags().BoolVar(&(req.PreserveWorkingDir), "savetempdir", false, "Save the temporary directory used by STI instead of deleting it")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Long:  "Display version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sti %s\n", version)
		},
	}

	stiCmd.AddCommand(versionCmd)

	buildCmd := &cobra.Command{
		Use:   "build SOURCE BUILD_IMAGE APP_IMAGE_TAG",
		Short: "Build an image",
		Long:  "Build an image",
		Run: func(cmd *cobra.Command, args []string) {
			// if we're not verbose, make sure the logger doesn't print out timestamps
			if !req.Verbose {
				log.SetFlags(0)
			}

			if len(args) == 0 {
				cmd.Usage()
				return
			}

			req.Source = args[0]
			req.BaseImage = args[1]
			req.Tag = args[2]
			req.Writer = os.Stdout

			envs, _ := parseEnvs(envString)
			req.Environment = envs

			res, err := sti.Build(req)
			if err != nil {
				fmt.Printf("An error occured: %s\n", err.Error())
				os.Exit(1)
			}

			for _, message := range res.Messages {
				fmt.Println(message)
			}
		},
	}
	buildCmd.Flags().BoolVar(&(req.Clean), "clean", false, "Perform a clean build")
	buildCmd.Flags().BoolVar(&(req.RemovePreviousImage), "rm", false, "Remove the previous image during incremental builds")
	buildCmd.Flags().StringVarP(&envString, "env", "e", "", "Specify an environment var NAME=VALUE,NAME2=VALUE2,...")
	buildCmd.Flags().StringVarP(&(req.Ref), "ref", "r", "", "Specify a ref to check-out")
	buildCmd.Flags().StringVar(&(req.CallbackUrl), "callbackUrl", "", "Specify a URL to invoke via HTTP POST upon build completion")
	buildCmd.Flags().StringVarP(&(req.ScriptsUrl), "scripts", "s", "", "Specify a URL for the assemble and run scripts")

	stiCmd.AddCommand(buildCmd)

	usageCmd := &cobra.Command{
		Use:   "usage BUILD_IMAGE",
		Short: "Print usage for assemble script associated with an image",
		Long:  "Print usage for assemble script associated with an image",
		Run: func(cmd *cobra.Command, args []string) {
			// if we're not verbose, make sure the logger doesn't print out timestamps
			if !req.Verbose {
				log.SetFlags(0)
			}

			if len(args) == 0 {
				cmd.Usage()
				return
			}

			req.BaseImage = args[0]
			req.Writer = os.Stdout

			envs, _ := parseEnvs(envString)
			req.Environment = envs

			err := sti.Usage(req)
			if err != nil {
				fmt.Printf("An error occured: %s\n", err.Error())
				os.Exit(1)
			}
		},
	}
	usageCmd.Flags().StringVarP(&envString, "env", "e", "", "Specify an environment var NAME=VALUE,NAME2=VALUE2,...")
	usageCmd.Flags().StringVarP(&(req.ScriptsUrl), "scripts", "s", "", "Specify a URL for the assemble and run scripts")

	stiCmd.AddCommand(usageCmd)

	stiCmd.Execute()
}

func main() {
	Execute()
}
