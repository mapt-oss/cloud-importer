package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	openshiftlocal "github.com/devtools-qe-incubator/cloud-importer/cmd/importer/cmd/openshift-local"
	"github.com/devtools-qe-incubator/cloud-importer/cmd/importer/cmd/replicate"
	"github.com/devtools-qe-incubator/cloud-importer/cmd/importer/cmd/rhelai"
	"github.com/devtools-qe-incubator/cloud-importer/cmd/importer/cmd/share"
	params "github.com/devtools-qe-incubator/cloud-importer/cmd/importer/params"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	commandName      = "cloud-importer"
	descriptionShort = "Importer tool"
	descriptionLong  = "Importer tool"

	defaultErrorExitCode = 1
)

// var (
// 	baseDir = filepath.Join(os.Getenv("HOME"), ".ci")
// 	logFile = "ci.log"
// )

var rootCmd = &cobra.Command{
	Use:   commandName,
	Short: descriptionShort,
	Long:  descriptionLong,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return runPrerun(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		runRoot()
		_ = cmd.Help()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func runPrerun(cmd *cobra.Command) error {
	// logging.InitLogrus(baseDir, logFile)
	return nil
}

func runRoot() {
	fmt.Println("No command given")
}

func init() {
	// Common flags
	flagSet := pflag.NewFlagSet(commandName, pflag.ExitOnError)
	params.AddCommonFlags(flagSet)
	rootCmd.PersistentFlags().AddFlagSet(flagSet)
	// Subcommands
	rootCmd.AddCommand(
		openshiftlocal.GetCmd(),
		rhelai.GetCmd(),
		share.GetCmd(),
		replicate.GetCmd(),
	)
}

func Execute() {
	attachMiddleware([]string{}, rootCmd)

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		runPostrun()
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(defaultErrorExitCode)
	}
	runPostrun()
}

func attachMiddleware(names []string, cmd *cobra.Command) {
	if cmd.HasSubCommands() {
		for _, command := range cmd.Commands() {
			attachMiddleware(append(names, cmd.Name()), command)
		}
	} else if cmd.RunE != nil {
		fullCmd := strings.Join(append(names, cmd.Name()), " ")
		src := cmd.RunE
		cmd.RunE = executeWithLogging(fullCmd, src)
	}
}

func executeWithLogging(fullCmd string, input func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// logging.Debugf("running '%s'", fullCmd)
		return input(cmd, args)
	}
}

func runPostrun() {
	// logging.CloseLogging()
}
