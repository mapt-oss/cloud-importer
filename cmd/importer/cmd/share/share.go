package share

import (
	params "github.com/devtools-qe-incubator/cloud-importer/cmd/importer/params"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cmd     = "share"
	cmdDesc = "share images"
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
	awsCMD                         = "aws"
	paramImageID                   = "image-id"
	paramImageIDDesc               = "image id to be shared"
	paramTargetAccountID           = "account-id"
	paramTargetAccountIDDesc       = "target account id to share the AMI to (mutually exclusive with organization-arn)"
	paramTargetOrganizationARN     = "organization-arn"
	paramTargetOrganizationARNDesc = "organization to share the AMI to (mutually exclusive with account-id)"
	paramArch                      = "arch"
	paramArchDesc                  = "image arch (x86_64 or arm64)"
)

func aws() *cobra.Command {
	c := &cobra.Command{
		Use:   awsCMD,
		Short: awsCMD,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := manager.ShareImage(
				&context.ContextArgs{
					BackedURL:  viper.GetString(params.BackedURL),
					Debug:      viper.IsSet(params.Debug),
					DebugLevel: viper.GetUint(params.DebugLevel),
				},
				viper.GetString(paramImageID),
				viper.GetString(paramTargetAccountID),
				viper.GetString(paramArch),
				viper.GetString(paramTargetOrganizationARN),
				manager.AWS); err != nil {
				return err
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(awsCMD, pflag.ExitOnError)
	flagSet.StringP(paramImageID, "", "", paramImageIDDesc)
	flagSet.StringP(paramTargetAccountID, "", "", paramTargetAccountIDDesc)
	flagSet.String(paramArch, "x86_64", paramArchDesc)
	flagSet.String(paramTargetOrganizationARN, "", paramTargetOrganizationARNDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
