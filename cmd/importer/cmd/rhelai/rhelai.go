package rhelai

import (
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
	return c
}

var (
	awsCMD             = "aws"
	paramImagePath     = "raw-image-path"
	paramImagePathDesc = "local path to the raw image"
	paramAMIName       = "ami-name"
	paramAMINameDesc   = "ami name once the image is upload"
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
