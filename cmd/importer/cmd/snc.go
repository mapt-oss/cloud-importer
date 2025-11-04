package cmd

import (
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	sncCmd     = "snc"
	sncCmdDesc = "snc openshift local image management"
)

func sncCmds() *cobra.Command {
	c := &cobra.Command{
		Use:   sncCmd,
		Short: sncCmdDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
	}
	c.AddCommand(sncCreate(awsCMD, manager.AWS))
	c.AddCommand(sncCreate(azureCMD, manager.AZURE))
	return c
}

var (
	paramBundleURL     = "bundle-uri"
	paramBundleURLDesc = "accessible uri to get the bundle"
	paramShasumURL     = "shasum-uri"
	paramShasumURLDesc = "accessible uri to get the shasum file to check bundle"
	paramArch          = "arch"
	paramArchDesc      = "architecture for the machine. Allowed x86_64 or arm64"
)

func sncCreate(cmd string, provider manager.Provider) *cobra.Command {
	c := &cobra.Command{
		Use:   cmd,
		Short: cmd,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := manager.SNC(
				contextArgs(),
				&manager.SNCArgs{
					BundleURI:    viper.GetString(paramBundleURL),
					ShasumURI:    viper.GetString(paramShasumURL),
					Arch:         viper.GetString(paramArch),
					ImageControl: imageControl(),
				},
				provider); err != nil {
				return err
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(cmd, pflag.ExitOnError)
	imageControlFlags(flagSet)
	flagSet.StringP(paramBundleURL, "", "", paramBundleURLDesc)
	flagSet.StringP(paramShasumURL, "", "", paramShasumURLDesc)
	flagSet.StringP(paramArch, "", "x86_64", paramArchDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
