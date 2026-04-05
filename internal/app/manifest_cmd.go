package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"forktool/internal/manifest"
	"forktool/pkg/cliui"
)

func (c *CLI) newManifestCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "manifest",
		Short: "Manifest management commands",
	}

	var manifestPath string
	validateCommand := &cobra.Command{
		Use:   "validate",
		Short: "Validate a feature manifest against the current schema",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workdir, err := filepath.Abs(".")
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			configPath := resolveConfigPath(workdir, c.configPath)
			workspace, found, err := loadWorkspaceIfExists(configPath)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			switch {
			case strings.TrimSpace(manifestPath) != "":
				manifestPath = resolveWorkspacePath(workdir, manifestPath)
			case found:
				manifestPath = resolveWorkspacePath(workspace.RepoRoot, workspace.Config.Manifest.Path)
			default:
				return withExitCode(fmt.Errorf("manifest path is required"), ExitInput)
			}

			_, result, err := manifest.LoadAndValidate(manifestPath)
			writeErr := cliui.WriteJSON(cmd.OutOrStdout(), result)
			if writeErr != nil {
				return writeErr
			}
			if err != nil {
				return withExitCode(err, ExitInput)
			}
			return nil
		},
	}

	validateCommand.Flags().StringVar(&manifestPath, "path", "", "manifest file path")
	command.AddCommand(validateCommand)
	return command
}
