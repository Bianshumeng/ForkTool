package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *CLI) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print ForkTool version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "forktool %s\n", Version)
			return err
		},
	}
}
