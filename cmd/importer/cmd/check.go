package cmd

import (
	"fmt"
	"os"

	"github.com/mapt-oss/cloud-importer/pkg/manager"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	checkCmd     = "check"
	checkCmdDesc = "check if a cloud image already exists"
)

func checkCmds() *cobra.Command {
	c := &cobra.Command{
		Use:   checkCmd,
		Short: checkCmdDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
	}
	c.AddCommand(checkCreate(awsCMD, manager.AWS))
	c.AddCommand(checkCreate(azureCMD, manager.AZURE))
	return c
}

func checkCreate(cmd string, provider manager.Provider) *cobra.Command {
	c := &cobra.Command{
		Use:   cmd,
		Short: cmd,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			imageName := viper.GetString(paramImageName)
			if imageName == "" {
				return fmt.Errorf("--%s is required", paramImageName)
			}
			exists, imageID, err := manager.CheckImageExists(imageName, provider)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking image %q: %v\n", imageName, err)
				os.Exit(2)
			}
			if exists {
				fmt.Printf("Image %q found: %s\n", imageName, imageID)
				return nil
			}
			fmt.Printf("Image %q not found\n", imageName)
			os.Exit(1)
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(cmd, pflag.ExitOnError)
	flagSet.StringP(paramImageName, "", "", paramImageNameDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
