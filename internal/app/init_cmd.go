package app

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"forktool/pkg/cliui"
)

func (c *CLI) newInitCommand() *cobra.Command {
	var repoKind string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize .forktool workspace files and directories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workdir, err := filepath.Abs(".")
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			workspace, err := initializeWorkspace(workdir, repoKind, Version, force)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			result := map[string]string{
				"configPath":   filepath.ToSlash(workspace.ConfigPath),
				"decisionFile": workspace.Config.DecisionFile.Path,
				"manifestPath": workspace.Config.Manifest.Path,
				"workspaceDir": ".forktool",
			}

			return cliui.WriteJSON(cmd.OutOrStdout(), result)
		},
	}

	cmd.Flags().StringVar(&repoKind, "repo-kind", "sub2api", "repository kind to initialize")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing workspace files")
	return cmd
}
