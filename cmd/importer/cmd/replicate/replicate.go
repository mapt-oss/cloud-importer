package replicate

import (
	params "github.com/devtools-qe-incubator/cloud-importer/cmd/importer/params"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cmd     = "replicate"
	cmdDesc = "copy image(s) to target region(s)"
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
	awsCMD                = "aws"
	paramAmiName          = "ami-name"
	paramImageNameDesc    = "AMI name to be replicated"
	paramTargetRegion     = "region"
	paramTargetRegionDesc = "target region ('all' to replicate across all regions)"
)

func aws() *cobra.Command {
	c := &cobra.Command{
		Use:   awsCMD,
		Short: awsCMD,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := manager.ReplicateImage(
				&context.ContextArgs{
					BackedURL:  viper.GetString(params.BackedURL),
					Debug:      viper.IsSet(params.Debug),
					DebugLevel: viper.GetUint(params.DebugLevel),
				},
				viper.GetString(paramAmiName),
				viper.GetString(paramTargetRegion),
				manager.AWS); err != nil {
				return err
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(awsCMD, pflag.ExitOnError)
	flagSet.StringP(paramAmiName, "", "", paramImageNameDesc)
	flagSet.StringP(paramTargetRegion, "", "", paramTargetRegionDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
