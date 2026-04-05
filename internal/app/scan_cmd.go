package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"forktool/internal/adapters/gox"
	"forktool/internal/baseline"
	"forktool/internal/discovery"
	"forktool/internal/ir"
	"forktool/internal/manifest"
	reporting "forktool/internal/report"
	"forktool/pkg/cliui"
	"forktool/pkg/model"
)

func (c *CLI) newScanCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "scan",
		Short: "Feature-chain scanning commands",
	}

	var featureID string
	var manifestPath string
	var localPath string
	var officialPath string
	var remoteURL string
	var tag string
	var commit string
	var outputDir string
	var remoteName string
	var formats []string

	featureCommand := &cobra.Command{
		Use:   "feature",
		Short: "Scan a single feature manifest entry and emit a placeholder report",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(featureID) == "" {
				return withExitCode(fmt.Errorf("feature id is required"), ExitInput)
			}

			workdir, err := filepath.Abs(".")
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			configPath := resolveConfigPath(workdir, c.configPath)
			workspace, found, err := loadWorkspaceIfExists(configPath)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			repoRoot := workdir
			config := model.WorkspaceConfig{}
			localRepo := model.RepoConfig{Kind: "fork"}
			officialRepo := model.RepoConfig{Kind: "official"}
			decisionPath := ""

			if found {
				repoRoot = workspace.RepoRoot
				config = workspace.Config
				localRepo = workspace.Config.LocalRepo
				officialRepo = workspace.Config.OfficialRepo
				decisionPath = resolveWorkspacePath(workspace.RepoRoot, workspace.Config.DecisionFile.Path)
			}

			switch {
			case strings.TrimSpace(manifestPath) != "":
				manifestPath = resolveWorkspacePath(workdir, manifestPath)
			case found:
				manifestPath = resolveWorkspacePath(workspace.RepoRoot, workspace.Config.Manifest.Path)
			default:
				return withExitCode(fmt.Errorf("manifest path is required"), ExitInput)
			}

			loadedManifest, validation, err := manifest.LoadAndValidate(manifestPath)
			if err != nil {
				_ = cliui.WriteJSON(cmd.OutOrStdout(), validation)
				return withExitCode(err, ExitInput)
			}

			feature, ok := manifest.FindFeature(loadedManifest, featureID)
			if !ok {
				return withExitCode(fmt.Errorf("feature %q not found in manifest", featureID), ExitInput)
			}

			if strings.TrimSpace(localPath) != "" {
				localRepo.Path = resolveWorkspacePath(workdir, localPath)
			} else {
				localRepo.Path = resolveWorkspacePath(repoRoot, localRepo.Path)
			}
			if strings.TrimSpace(officialPath) != "" {
				officialRepo.Path = resolveWorkspacePath(workdir, officialPath)
			} else {
				officialRepo.Path = resolveWorkspacePath(repoRoot, officialRepo.Path)
			}
			if strings.TrimSpace(remoteURL) != "" {
				officialRepo.RemoteURL = remoteURL
			}
			if strings.TrimSpace(tag) != "" {
				officialRepo.Tag = tag
			}
			if strings.TrimSpace(commit) != "" {
				officialRepo.Commit = commit
			}

			baselineStatus := "skipped"
			var baselineResult *model.BaselineVerificationResult
			if shouldVerifyBaseline(officialRepo) {
				verified, verifyErr := baseline.Verify(context.Background(), baseline.VerifyInput{
					Official:   officialRepo,
					RemoteName: remoteName,
				})
				if verifyErr != nil {
					return withExitCode(verifyErr, ExitBaseline)
				}
				baselineResult = &verified
				if !verified.Valid {
					_ = cliui.WriteJSON(cmd.OutOrStdout(), verified)
					return withExitCode(fmt.Errorf("baseline verification failed"), ExitBaseline)
				}
				officialRepo.Commit = verified.Official.Commit
				baselineStatus = "verified"
			}

			manager := discovery.NewManager(gox.New())
			var localNodes []model.ChainNode
			if repoExists(localRepo.Path) {
				localNodes, err = manager.Discover(context.Background(), discovery.DiscoverRequest{
					RepoRoot: localRepo.Path,
					Feature:  feature,
					RepoSide: "local",
				})
				if err != nil {
					return err
				}
			}

			var officialNodes []model.ChainNode
			if repoExists(officialRepo.Path) {
				officialNodes, err = manager.Discover(context.Background(), discovery.DiscoverRequest{
					RepoRoot: officialRepo.Path,
					Feature:  feature,
					RepoSide: "official",
				})
				if err != nil {
					return err
				}
			}

			featureChain := ir.NewFeatureChain(feature, localNodes, officialNodes)
			runID := newRunID(officialRepo.Tag, officialRepo.Commit)
			runDir, err := prepareRunDirectory(repoRoot, config, outputDir, runID)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			discoveryMode := "manifest-only"
			if len(featureChain.LocalNodes) > 0 || len(featureChain.OfficialNodes) > 0 {
				discoveryMode = "manifest+gox-skeleton"
			}

			report := buildFeaturePlaceholderReport(runID, loadedManifest.Version, localRepo, officialRepo, featureChain)
			reporting.PopulateSummary(&report)

			contextPath, err := writeRunContext(runDir, model.RunContext{
				RunID:            runID,
				GeneratedAt:      time.Now().UTC(),
				ToolVersion:      Version,
				ManifestPath:     filepath.ToSlash(manifestPath),
				DecisionFilePath: filepath.ToSlash(decisionPath),
				OutputDir:        filepath.ToSlash(runDir),
				LocalRepo: model.RepoSnapshot{
					Path: filepath.ToSlash(localRepo.Path),
					Kind: localRepo.Kind,
				},
				OfficialRepo: model.RepoSnapshot{
					Path:      filepath.ToSlash(officialRepo.Path),
					Kind:      officialRepo.Kind,
					RemoteURL: officialRepo.RemoteURL,
					Tag:       officialRepo.Tag,
					Commit:    officialRepo.Commit,
				},
				Baseline: baselineResult,
			})
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			reportFiles, err := reporting.WriteAll(report, runDir, resolveFormats(formats, config))
			if err != nil {
				return err
			}

			result := model.ScanFeatureResult{
				FeatureID:      feature.ID,
				RunID:          runID,
				OutputDir:      filepath.ToSlash(runDir),
				ContextPath:    contextPath,
				ReportFiles:    reportFiles,
				BaselineStatus: baselineStatus,
				DiscoveryMode:  discoveryMode,
			}
			if err := cliui.WriteJSON(cmd.OutOrStdout(), result); err != nil {
				return err
			}

			if reporting.HighestSeverity(report) != "" {
				return withExitCode(fmt.Errorf("scan reported high-risk findings"), ExitFindings)
			}
			return nil
		},
	}

	featureCommand.Flags().StringVar(&featureID, "feature", "", "feature id from the manifest")
	featureCommand.Flags().StringVar(&manifestPath, "manifest", "", "manifest file path")
	featureCommand.Flags().StringVar(&localPath, "local", "", "local fork repository path")
	featureCommand.Flags().StringVar(&officialPath, "official", "", "official repository path")
	featureCommand.Flags().StringVar(&remoteURL, "remote-url", "", "expected official remote URL")
	featureCommand.Flags().StringVar(&tag, "tag", "", "expected official tag")
	featureCommand.Flags().StringVar(&commit, "commit", "", "expected official commit")
	featureCommand.Flags().StringVar(&outputDir, "out", "", "output directory for reports")
	featureCommand.Flags().StringVar(&remoteName, "remote-name", "origin", "git remote name to validate")
	featureCommand.Flags().StringSliceVar(&formats, "format", nil, "report formats to emit (md,json)")

	command.AddCommand(featureCommand)
	return command
}

func shouldVerifyBaseline(officialRepo model.RepoConfig) bool {
	return strings.TrimSpace(officialRepo.Path) != "" &&
		strings.TrimSpace(officialRepo.RemoteURL) != "" &&
		(strings.TrimSpace(officialRepo.Tag) != "" || strings.TrimSpace(officialRepo.Commit) != "")
}

func repoExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func buildFeaturePlaceholderReport(runID string, manifestVersion int, localRepo, officialRepo model.RepoConfig, featureChain model.FeatureChain) model.AuditReport {
	report := model.AuditReport{
		RunID:           runID,
		GeneratedAt:     time.Now().UTC(),
		ManifestVersion: manifestVersion,
		LocalRepo: model.RepoSnapshot{
			Path: filepath.ToSlash(localRepo.Path),
			Kind: localRepo.Kind,
		},
		OfficialRepo: model.RepoSnapshot{
			Path:      filepath.ToSlash(officialRepo.Path),
			Kind:      officialRepo.Kind,
			RemoteURL: officialRepo.RemoteURL,
			Tag:       officialRepo.Tag,
			Commit:    officialRepo.Commit,
		},
		Features: []model.FeatureReport{
			{
				ID:            featureChain.ID,
				Name:          featureChain.Name,
				RiskLevel:     featureChain.RiskLevel,
				Status:        "placeholder",
				SemanticRules: featureChain.SemanticRules,
				LocalNodes:    featureChain.LocalNodes,
				OfficialNodes: featureChain.OfficialNodes,
				Notes: []string{
					"Go Adapter skeleton is active and currently emits manifest-driven route, handler/service symbol, and test nodes.",
					"Rule engine findings are not implemented yet; this report verifies manifest loading, baseline plumbing, IR assembly, and report output.",
				},
			},
		},
	}
	return report
}
