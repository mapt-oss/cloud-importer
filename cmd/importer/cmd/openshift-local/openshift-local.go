package openshiftlocal

import (
	params "github.com/devtools-qe-incubator/cloud-importer/cmd/importer/params"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/bundle"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cmd     = "openshift-local"
	cmdDesc = "openshift local import"
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
	paramBundleURL     = "bundle-url"
	paramBundleURLDesc = "accessible url to get the bundle"
	paramShasumURL     = "shasum-url"
	paramShasumURLDesc = "accessible url to get the shasum file to check bundle"
	paramArch          = "arch"
	paramArchDesc      = "architecture for the machine. Allowed x86_64 or arm64"
	paramReplicate     = "replicate"
	paramReplicateDesc = "provide a list of regions to replicate the ami to, or 'all' to replicate to all available regions"
)

func aws() *cobra.Command {
	c := &cobra.Command{
		Use:   awsCMD,
		Short: awsCMD,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := manager.OpenshiftLocal(
				&context.ContextArgs{
					BackedURL:  viper.GetString(params.BackedURL),
					Output:     viper.GetString(params.Output),
					Debug:      viper.IsSet(params.Debug),
					DebugLevel: viper.GetUint(params.DebugLevel),
				},
				viper.GetString(paramBundleURL),
				viper.GetString(paramShasumURL),
				viper.GetString(paramArch),
				manager.AWS); err != nil {
				return err
			}

			if regions := viper.GetStringSlice(paramReplicate); len(regions) > 0 {
				if err := manager.ReplicateImage(
					&context.ContextArgs{
						BackedURL:  viper.GetString(params.BackedURL),
						Debug:      viper.IsSet(params.Debug),
						DebugLevel: viper.GetUint(params.DebugLevel),
					},
					bundle.GetAmiNameFromBundleURLandArch(viper.GetString(paramBundleURL), viper.GetString(paramArch)),
					regions,
					manager.AWS); err != nil {
					return err
				}
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(awsCMD, pflag.ExitOnError)
	flagSet.StringP(params.Output, "", "", params.OutputDesc)
	flagSet.StringP(paramBundleURL, "", "", paramBundleURLDesc)
	flagSet.StringP(paramShasumURL, "", "", paramShasumURLDesc)
	flagSet.StringP(paramArch, "", "", paramArchDesc)
	flagSet.StringSliceP(paramReplicate, "", []string{}, paramReplicateDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
