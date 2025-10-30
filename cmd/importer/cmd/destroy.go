package cmd

import (
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	destroyCmd     = "destroy"
	destroyCmdDesc = "destroy import"
)

func destroy() *cobra.Command {
	c := &cobra.Command{
		Use:   destroyCmd,
		Short: destroyCmdDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := manager.Destoy(
				contextArgs()); err != nil {
				return err
			}
			return nil
		},
	}
	flagSet := pflag.NewFlagSet(destroyCmd, pflag.ExitOnError)
	c.PersistentFlags().AddFlagSet(flagSet)
	return c
}
