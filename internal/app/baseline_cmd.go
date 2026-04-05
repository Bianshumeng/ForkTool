package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"forktool/internal/baseline"
	"forktool/pkg/cliui"
	"forktool/pkg/model"
)

type baselineVerifyOutput struct {
	RunID       string                           `json:"runId"`
	ContextPath string                           `json:"contextPath"`
	Result      model.BaselineVerificationResult `json:"result"`
}

func (c *CLI) newBaselineCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "baseline",
		Short: "Baseline management commands",
	}

	var officialPath string
	var remoteURL string
	var tag string
	var commit string
	var remoteName string

	verifyCommand := &cobra.Command{
		Use:   "verify",
		Short: "Verify official repository path, remote URL, and baseline revision",
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

			official := model.RepoConfig{Kind: "official"}
			local := model.RepoConfig{Kind: "fork"}
			outputConfig := model.WorkspaceConfig{}
			repoRoot := workdir
			manifestPath := ""
			decisionPath := ""

			if found {
				official = workspace.Config.OfficialRepo
				local = workspace.Config.LocalRepo
				outputConfig = workspace.Config
				repoRoot = workspace.RepoRoot
				manifestPath = resolveWorkspacePath(workspace.RepoRoot, workspace.Config.Manifest.Path)
				decisionPath = resolveWorkspacePath(workspace.RepoRoot, workspace.Config.DecisionFile.Path)
			}

			if strings.TrimSpace(officialPath) != "" {
				official.Path = resolveWorkspacePath(workdir, officialPath)
			} else {
				official.Path = resolveWorkspacePath(repoRoot, official.Path)
			}
			if strings.TrimSpace(remoteURL) != "" {
				official.RemoteURL = remoteURL
			}
			if strings.TrimSpace(tag) != "" {
				official.Tag = tag
			}
			if strings.TrimSpace(commit) != "" {
				official.Commit = commit
			}

			result, err := baseline.Verify(context.Background(), baseline.VerifyInput{
				Official:   official,
				RemoteName: remoteName,
			})
			if err != nil {
				return withExitCode(err, ExitBaseline)
			}

			runID := newRunID(official.Tag, official.Commit)
			runDir, err := prepareRunDirectory(repoRoot, outputConfig, "", runID)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			contextPath, err := writeRunContext(runDir, model.RunContext{
				RunID:            runID,
				GeneratedAt:      time.Now().UTC(),
				ToolVersion:      Version,
				ManifestPath:     filepath.ToSlash(manifestPath),
				DecisionFilePath: filepath.ToSlash(decisionPath),
				OutputDir:        filepath.ToSlash(runDir),
				LocalRepo: model.RepoSnapshot{
					Path: filepath.ToSlash(resolveWorkspacePath(repoRoot, local.Path)),
					Kind: local.Kind,
				},
				OfficialRepo: result.Official,
				Baseline:     &result,
			})
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			output := baselineVerifyOutput{
				RunID:       runID,
				ContextPath: contextPath,
				Result:      result,
			}

			if err := cliui.WriteJSON(cmd.OutOrStdout(), output); err != nil {
				return err
			}

			if !result.Valid {
				return withExitCode(fmt.Errorf("baseline verification failed"), ExitBaseline)
			}
			return nil
		},
	}

	verifyCommand.Flags().StringVar(&officialPath, "official", "", "official repository path")
	verifyCommand.Flags().StringVar(&remoteURL, "remote-url", "", "expected official remote URL")
	verifyCommand.Flags().StringVar(&tag, "tag", "", "expected official tag")
	verifyCommand.Flags().StringVar(&commit, "commit", "", "expected official commit")
	verifyCommand.Flags().StringVar(&remoteName, "remote-name", "origin", "git remote name to validate")

	command.AddCommand(verifyCommand)
	return command
}
