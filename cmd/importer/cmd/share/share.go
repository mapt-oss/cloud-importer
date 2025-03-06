package share

import (
	params "github.com/devtools-qe-incubator/cloud-importer/cmd/importer/constants"
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
	awsCMD                   = "aws"
	paramImageID             = "image-id"
	paramImageIDDesc         = "image id to be shared"
	paramTargetAccountID     = "account-id"
	paramTargetAccountIDDesc = "target account id"
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
				manager.AWS); err != nil {
				return err
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(awsCMD, pflag.ExitOnError)
	flagSet.StringP(paramImageID, "", "", paramImageIDDesc)
	flagSet.StringP(paramTargetAccountID, "", "", paramTargetAccountIDDesc)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
