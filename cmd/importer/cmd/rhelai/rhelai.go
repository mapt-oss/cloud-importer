package rhelai

import (
	"fmt"
	"os"

	params "github.com/devtools-qe-incubator/cloud-importer/cmd/importer/params"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cmd     = "rhelai"
	cmdDesc = "rhel ai import"
)

func GetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   cmd,
		Short: cmdDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
	}
	c.AddCommand(aws())
	c.AddCommand(azure())
	return c
}

var (
	awsCMD             = "aws"
	paramImagePath     = "raw-image-path"
	paramImagePathDesc = "local path to the raw image"
	paramAMIName       = "ami-name"
	paramAMINameDesc   = "ami name once the image is upload"

	azureCMD           = "azure"
	paramImageName     = "image-name"
	paramImageNameDesc = "name for the image in azure"
)

func aws() *cobra.Command {
	c := &cobra.Command{
		Use:   awsCMD,
		Short: awsCMD,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := manager.RHELAI(
				&context.ContextArgs{
					BackedURL:  viper.GetString(params.BackedURL),
					Output:     viper.GetString(params.Output),
					Debug:      viper.IsSet(params.Debug),
					DebugLevel: viper.GetUint(params.DebugLevel),
				},
				viper.GetString(paramImagePath),
				viper.GetString(paramAMIName),
				manager.AWS); err != nil {
				return err
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(awsCMD, pflag.ExitOnError)
	flagSet.StringP(params.Output, "", "", params.OutputDesc)
	flagSet.StringP(paramImagePath, "", "", paramImagePathDesc)
	flagSet.StringP(paramAMIName, "", "", paramAMINameDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}

func azure() *cobra.Command {

	c := &cobra.Command{
		Use:   azureCMD,
		Short: azureCMD,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}

			subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
			if subscriptionID == "" {
				return fmt.Errorf("AZURE_SUBSCRIPTION_ID environment variable not set")
			}

			resourceGroup := os.Getenv("AZURE_RESOURCE_GROUP")
			if resourceGroup == "" {
				return fmt.Errorf("AZURE_RESOURCE_GROUP environment variable not set")
			}

			location := os.Getenv("AZURE_LOCATION")
			if location == "" {
				return fmt.Errorf("AZURE_LOCATION environment variable not set")
			}

			if err := manager.RHELAIOnAzure(
				&context.ContextArgs{
					BackedURL:  viper.GetString(params.BackedURL),
					Output:     viper.GetString(params.Output),
					Debug:      viper.IsSet(params.Debug),
					DebugLevel: viper.GetUint(params.DebugLevel),
				},
				subscriptionID,
				resourceGroup,
				location,
				viper.GetString(paramImagePath),
				viper.GetString(paramImageName),
				map[string]string{
					"CreatedBy": "cloud-importer",
				},
				manager.AZURE); err != nil {
				return err
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(azureCMD, pflag.ExitOnError)
	flagSet.StringP(params.Output, "", "", params.OutputDesc)
	flagSet.StringP(paramImagePath, "", "", paramImagePathDesc)
	flagSet.StringP(paramImageName, "", "", paramImageNameDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
