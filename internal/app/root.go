package app

import "github.com/spf13/cobra"

type CLI struct {
	configPath string
}

func NewRootCommand() *cobra.Command {
	cli := &CLI{}

	root := &cobra.Command{
		Use:           "forktool",
		Short:         "ForkTool is a feature-chain official sync auditor.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&cli.configPath, "config", "", "workspace config file path")

	root.AddCommand(
		cli.newVersionCommand(),
		cli.newInitCommand(),
		cli.newDoctorCommand(),
		cli.newManifestCommand(),
		cli.newBaselineCommand(),
		cli.newScanCommand(),
		cli.newReportCommand(),
	)

	return root
}

func Execute() error {
	return NewRootCommand().Execute()
}
