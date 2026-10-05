package cmd

import (
	"github.com/spf13/cobra"
)

const (
	ibmCmd     = "ibm"
	ibmCmdDesc = "IBM Cloud-specific operations"
)

func ibmCmds() *cobra.Command {
	c := &cobra.Command{
		Use:   ibmCmd,
		Short: ibmCmdDesc,
	}
	return c
}
