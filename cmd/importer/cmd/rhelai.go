package cmd

import (
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	rhelaiCmd     = "rhelai"
	rhelaiCmdDesc = "rhel ai import"
)

func rhelaiCmds() *cobra.Command {
	c := &cobra.Command{
		Use:   rhelaiCmd,
		Short: rhelaiCmdDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
	}
	c.AddCommand(rhelaiCreate(awsCMD, manager.AWS))
	c.AddCommand(rhelaiCreate(azureCMD, manager.AZURE))
	return c
}

var (
	paramImagePath     = "image-path"
	paramImagePathDesc = "local path to the image"
	paramImageName     = "image-name"
	paramImageNameDesc = "image name once the image is upload"
)

func rhelaiCreate(cmd string, provider manager.Provider) *cobra.Command {
	c := &cobra.Command{
		Use:   cmd,
		Short: cmd,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := manager.RHELAI(
				contextArgs(),
				&manager.RHELAIArgs{
					ImageFilepath: viper.GetString(paramImagePath),
					ImageName:     viper.GetString(paramImageName),
					ImageControl:  imageControl(),
				},
				provider); err != nil {
				return err
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(cmd, pflag.ExitOnError)
	imageControlFlags(flagSet)
	flagSet.StringP(paramImagePath, "", "", paramImagePathDesc)
	flagSet.StringP(paramImageName, "", "", paramImageNameDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
