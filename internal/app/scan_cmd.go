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
	"forktool/internal/decision"
	"forktool/internal/discovery"
	"forktool/internal/ir"
	"forktool/internal/manifest"
	reporting "forktool/internal/report"
	"forktool/internal/rules"
	"forktool/pkg/cliui"
	"forktool/pkg/model"
)

type scanOptions struct {
	manifestPath string
	decisionPath string
	localPath    string
	officialPath string
	remoteURL    string
	tag          string
	commit       string
	outputDir    string
	remoteName   string
	formats      []string
}

type scanEnvironment struct {
	repoRoot       string
	config         model.WorkspaceConfig
	manifest       model.FeatureManifest
	manifestPath   string
	decisionPath   string
	localRepo      model.RepoConfig
	officialRepo   model.RepoConfig
	baselineStatus string
	baselineResult *model.BaselineVerificationResult
}

func (c *CLI) newScanCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "scan",
		Short: "Feature-chain scanning commands",
	}

	command.AddCommand(
		c.newScanFeatureCommand(),
		c.newScanReleaseCommand(),
	)

	return command
}

func (c *CLI) newScanFeatureCommand() *cobra.Command {
	var featureID string
	options := bindScanOptions()

	command := &cobra.Command{
		Use:   "feature",
		Short: "Scan a single feature manifest entry and emit an audit report",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(featureID) == "" {
				return withExitCode(fmt.Errorf("feature id is required"), ExitInput)
			}

			workdir, err := filepath.Abs(".")
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			environment, err := c.loadScanEnvironment(workdir, *options)
			if err != nil {
				return err
			}

			feature, ok := manifest.FindFeature(environment.manifest, featureID)
			if !ok {
				return withExitCode(fmt.Errorf("feature %q not found in manifest", featureID), ExitInput)
			}

			runID := newRunID(environment.officialRepo.Tag, environment.officialRepo.Commit)
			runDir, err := prepareRunDirectory(environment.repoRoot, environment.config, options.outputDir, runID)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			featureReport, featureChain, err := c.scanFeature(context.Background(), environment, feature)
			if err != nil {
				return err
			}

			report := buildAuditReport(runID, environment, environment.manifest.Version, []model.FeatureReport{featureReport})
			reporting.PopulateSummary(&report)

			contextPath, err := writeRunContext(runDir, model.RunContext{
				RunID:            runID,
				GeneratedAt:      time.Now().UTC(),
				ToolVersion:      Version,
				ManifestPath:     filepath.ToSlash(environment.manifestPath),
				DecisionFilePath: filepath.ToSlash(environment.decisionPath),
				OutputDir:        filepath.ToSlash(runDir),
				LocalRepo: model.RepoSnapshot{
					Path: filepath.ToSlash(environment.localRepo.Path),
					Kind: environment.localRepo.Kind,
				},
				OfficialRepo: model.RepoSnapshot{
					Path:      filepath.ToSlash(environment.officialRepo.Path),
					Kind:      environment.officialRepo.Kind,
					RemoteURL: environment.officialRepo.RemoteURL,
					Tag:       environment.officialRepo.Tag,
					Commit:    environment.officialRepo.Commit,
				},
				Baseline: environment.baselineResult,
			})
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			reportFiles, err := reporting.WriteAll(report, runDir, resolveFormats(options.formats, environment.config))
			if err != nil {
				return err
			}

			result := model.ScanFeatureResult{
				FeatureID:         feature.ID,
				RunID:             runID,
				OutputDir:         filepath.ToSlash(runDir),
				ContextPath:       contextPath,
				ReportFiles:       reportFiles,
				BaselineStatus:    environment.baselineStatus,
				DiscoveryMode:     detectDiscoveryMode(featureChain),
				LocalNodeCount:    len(featureChain.LocalNodes),
				OfficialNodeCount: len(featureChain.OfficialNodes),
				FindingCount:      len(featureReport.Findings),
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

	command.Flags().StringVar(&featureID, "feature", "", "feature id from the manifest")
	applyScanFlags(command, options)
	return command
}

func (c *CLI) newScanReleaseCommand() *cobra.Command {
	options := bindScanOptions()
	var criticalOnly bool
	var selectedFeatures []string

	command := &cobra.Command{
		Use:   "release",
		Short: "Scan a batch of features and emit an aggregated release audit report",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workdir, err := filepath.Abs(".")
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			environment, err := c.loadScanEnvironment(workdir, *options)
			if err != nil {
				return err
			}

			features := selectFeatures(environment.manifest.Features, criticalOnly, selectedFeatures)
			if len(features) == 0 {
				return withExitCode(fmt.Errorf("no features selected for release scan"), ExitInput)
			}

			runID := newRunID(environment.officialRepo.Tag, environment.officialRepo.Commit)
			runDir, err := prepareRunDirectory(environment.repoRoot, environment.config, options.outputDir, runID)
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			featureReports := make([]model.FeatureReport, 0, len(features))
			totalFindings := 0
			for _, feature := range features {
				featureReport, _, err := c.scanFeature(context.Background(), environment, feature)
				if err != nil {
					return err
				}
				totalFindings += len(featureReport.Findings)
				featureReports = append(featureReports, featureReport)
			}

			report := buildAuditReport(runID, environment, environment.manifest.Version, featureReports)
			reporting.PopulateSummary(&report)

			contextPath, err := writeRunContext(runDir, model.RunContext{
				RunID:            runID,
				GeneratedAt:      time.Now().UTC(),
				ToolVersion:      Version,
				ManifestPath:     filepath.ToSlash(environment.manifestPath),
				DecisionFilePath: filepath.ToSlash(environment.decisionPath),
				OutputDir:        filepath.ToSlash(runDir),
				LocalRepo: model.RepoSnapshot{
					Path: filepath.ToSlash(environment.localRepo.Path),
					Kind: environment.localRepo.Kind,
				},
				OfficialRepo: model.RepoSnapshot{
					Path:      filepath.ToSlash(environment.officialRepo.Path),
					Kind:      environment.officialRepo.Kind,
					RemoteURL: environment.officialRepo.RemoteURL,
					Tag:       environment.officialRepo.Tag,
					Commit:    environment.officialRepo.Commit,
				},
				Baseline: environment.baselineResult,
			})
			if err != nil {
				return withExitCode(err, ExitInput)
			}

			reportFiles, err := reporting.WriteAll(report, runDir, resolveFormats(options.formats, environment.config))
			if err != nil {
				return err
			}

			result := model.ScanReleaseResult{
				RunID:           runID,
				OutputDir:       filepath.ToSlash(runDir),
				ContextPath:     contextPath,
				ReportFiles:     reportFiles,
				BaselineStatus:  environment.baselineStatus,
				FeaturesScanned: len(featureReports),
				FindingCount:    totalFindings,
			}

			if err := cliui.WriteJSON(cmd.OutOrStdout(), result); err != nil {
				return err
			}

			if reporting.HighestSeverity(report) != "" {
				return withExitCode(fmt.Errorf("release scan reported high-risk findings"), ExitFindings)
			}
			return nil
		},
	}

	command.Flags().BoolVar(&criticalOnly, "critical-only", false, "scan only critical features from the manifest")
	command.Flags().StringSliceVar(&selectedFeatures, "feature", nil, "scan only the selected feature ids")
	applyScanFlags(command, options)
	return command
}

func bindScanOptions() *scanOptions {
	return &scanOptions{
		remoteName: "origin",
	}
}

func applyScanFlags(command *cobra.Command, options *scanOptions) {
	command.Flags().StringVar(&options.manifestPath, "manifest", "", "manifest file path")
	command.Flags().StringVar(&options.decisionPath, "decision-file", "", "decision file path")
	command.Flags().StringVar(&options.localPath, "local", "", "local fork repository path")
	command.Flags().StringVar(&options.officialPath, "official", "", "official repository path")
	command.Flags().StringVar(&options.remoteURL, "remote-url", "", "expected official remote URL")
	command.Flags().StringVar(&options.tag, "tag", "", "expected official tag")
	command.Flags().StringVar(&options.commit, "commit", "", "expected official commit")
	command.Flags().StringVar(&options.outputDir, "out", "", "output directory for reports")
	command.Flags().StringVar(&options.remoteName, "remote-name", "origin", "git remote name to validate")
	command.Flags().StringSliceVar(&options.formats, "format", nil, "report formats to emit (md,json)")
}

func (c *CLI) loadScanEnvironment(workdir string, options scanOptions) (scanEnvironment, error) {
	configPath := resolveConfigPath(workdir, c.configPath)
	workspace, found, err := loadWorkspaceIfExists(configPath)
	if err != nil {
		return scanEnvironment{}, withExitCode(err, ExitInput)
	}

	environment := scanEnvironment{
		repoRoot: workdir,
		config:   model.WorkspaceConfig{},
		localRepo: model.RepoConfig{
			Kind: "fork",
		},
		officialRepo: model.RepoConfig{
			Kind: "official",
		},
		baselineStatus: "skipped",
	}

	if found {
		environment.repoRoot = workspace.RepoRoot
		environment.config = workspace.Config
		environment.localRepo = workspace.Config.LocalRepo
		environment.officialRepo = workspace.Config.OfficialRepo
		environment.decisionPath = resolveWorkspacePath(workspace.RepoRoot, workspace.Config.DecisionFile.Path)
	}

	switch {
	case strings.TrimSpace(options.manifestPath) != "":
		environment.manifestPath = resolveWorkspacePath(workdir, options.manifestPath)
	case found:
		environment.manifestPath = resolveWorkspacePath(workspace.RepoRoot, workspace.Config.Manifest.Path)
	default:
		return scanEnvironment{}, withExitCode(fmt.Errorf("manifest path is required"), ExitInput)
	}

	if strings.TrimSpace(options.decisionPath) != "" {
		environment.decisionPath = resolveWorkspacePath(workdir, options.decisionPath)
	}

	loadedManifest, validation, err := manifest.LoadAndValidate(environment.manifestPath)
	if err != nil {
		return scanEnvironment{}, withExitCode(fmt.Errorf("manifest validation failed: %s", strings.Join(validation.Errors, "; ")), ExitInput)
	}
	environment.manifest = loadedManifest

	if strings.TrimSpace(options.localPath) != "" {
		environment.localRepo.Path = resolveWorkspacePath(workdir, options.localPath)
	} else {
		environment.localRepo.Path = resolveWorkspacePath(environment.repoRoot, environment.localRepo.Path)
	}

	if strings.TrimSpace(options.officialPath) != "" {
		environment.officialRepo.Path = resolveWorkspacePath(workdir, options.officialPath)
	} else {
		environment.officialRepo.Path = resolveWorkspacePath(environment.repoRoot, environment.officialRepo.Path)
	}
	if strings.TrimSpace(options.remoteURL) != "" {
		environment.officialRepo.RemoteURL = options.remoteURL
	}
	if strings.TrimSpace(options.tag) != "" {
		environment.officialRepo.Tag = options.tag
	}
	if strings.TrimSpace(options.commit) != "" {
		environment.officialRepo.Commit = options.commit
	}

	if shouldVerifyBaseline(environment.officialRepo) {
		verified, verifyErr := baseline.Verify(context.Background(), baseline.VerifyInput{
			Official:   environment.officialRepo,
			RemoteName: options.remoteName,
		})
		if verifyErr != nil {
			return scanEnvironment{}, withExitCode(verifyErr, ExitBaseline)
		}
		environment.baselineResult = &verified
		if !verified.Valid {
			return scanEnvironment{}, withExitCode(fmt.Errorf("baseline verification failed"), ExitBaseline)
		}
		environment.officialRepo.Commit = verified.Official.Commit
		environment.baselineStatus = "verified"
	}

	return environment, nil
}

func (c *CLI) scanFeature(ctx context.Context, environment scanEnvironment, feature model.ManifestFeature) (model.FeatureReport, model.FeatureChain, error) {
	decisionHints, err := loadDecisionHints(environment.decisionPath, feature.ID)
	if err != nil {
		return model.FeatureReport{}, model.FeatureChain{}, withExitCode(err, ExitInput)
	}

	manager := discovery.NewManager(gox.New())
	var localNodes []model.ChainNode
	if repoExists(environment.localRepo.Path) {
		localNodes, err = manager.Discover(ctx, discovery.DiscoverRequest{
			RepoRoot: environment.localRepo.Path,
			Feature:  feature,
			RepoSide: "local",
		})
		if err != nil {
			return model.FeatureReport{}, model.FeatureChain{}, err
		}
	}

	var officialNodes []model.ChainNode
	if repoExists(environment.officialRepo.Path) {
		officialNodes, err = manager.Discover(ctx, discovery.DiscoverRequest{
			RepoRoot: environment.officialRepo.Path,
			Feature:  feature,
			RepoSide: "official",
		})
		if err != nil {
			return model.FeatureReport{}, model.FeatureChain{}, err
		}
	}

	featureChain := ir.NewFeatureChain(feature, localNodes, officialNodes, decisionHints)
	findings, err := rules.NewEngine(environment.localRepo.Path, environment.officialRepo.Path).Apply(ctx, featureChain, decisionHints)
	if err != nil {
		return model.FeatureReport{}, model.FeatureChain{}, err
	}

	notes := []string{
		fmt.Sprintf("Go Adapter extracted %d local node(s) and %d official node(s).", countExtractedNodes(featureChain.LocalNodes), countExtractedNodes(featureChain.OfficialNodes)),
		fmt.Sprintf("Applied %d semantic rule(s): %s", len(feature.SemanticRules), strings.Join(feature.SemanticRules, ", ")),
	}
	if len(decisionHints) > 0 {
		notes = append(notes, fmt.Sprintf("Loaded %d decision hint(s) from decision file.", len(decisionHints)))
	}
	if len(findings) == 0 {
		notes = append(notes, "No semantic drift detected by the currently supported deterministic rules.")
	}

	return model.FeatureReport{
		ID:            featureChain.ID,
		Name:          featureChain.Name,
		RiskLevel:     featureChain.RiskLevel,
		Status:        featureStatus(featureChain, findings),
		SemanticRules: featureChain.SemanticRules,
		LocalNodes:    featureChain.LocalNodes,
		OfficialNodes: featureChain.OfficialNodes,
		Findings:      findings,
		Notes:         notes,
	}, featureChain, nil
}

func loadDecisionHints(decisionPath, featureID string) ([]model.DecisionHint, error) {
	if strings.TrimSpace(decisionPath) == "" {
		return nil, nil
	}
	if _, err := os.Stat(decisionPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat decision file: %w", err)
	}

	loaded, err := decision.Load(decisionPath)
	if err != nil {
		return nil, err
	}

	return decision.FilterForFeature(loaded, featureID), nil
}

func selectFeatures(features []model.ManifestFeature, criticalOnly bool, selected []string) []model.ManifestFeature {
	selectedSet := make(map[string]struct{}, len(selected))
	for _, featureID := range selected {
		selectedSet[featureID] = struct{}{}
	}

	filtered := make([]model.ManifestFeature, 0, len(features))
	for _, feature := range features {
		if criticalOnly && !strings.EqualFold(feature.RiskLevel, "critical") {
			continue
		}
		if len(selectedSet) > 0 {
			if _, ok := selectedSet[feature.ID]; !ok {
				continue
			}
		}
		filtered = append(filtered, feature)
	}
	return filtered
}

func buildAuditReport(runID string, environment scanEnvironment, manifestVersion int, featureReports []model.FeatureReport) model.AuditReport {
	return model.AuditReport{
		RunID:           runID,
		GeneratedAt:     time.Now().UTC(),
		ManifestVersion: manifestVersion,
		LocalRepo: model.RepoSnapshot{
			Path: filepath.ToSlash(environment.localRepo.Path),
			Kind: environment.localRepo.Kind,
		},
		OfficialRepo: model.RepoSnapshot{
			Path:      filepath.ToSlash(environment.officialRepo.Path),
			Kind:      environment.officialRepo.Kind,
			RemoteURL: environment.officialRepo.RemoteURL,
			Tag:       environment.officialRepo.Tag,
			Commit:    environment.officialRepo.Commit,
		},
		Features: featureReports,
	}
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

func detectDiscoveryMode(featureChain model.FeatureChain) string {
	if countExtractedNodes(featureChain.LocalNodes)+countExtractedNodes(featureChain.OfficialNodes) > 0 {
		return "gox-ast"
	}
	if len(featureChain.LocalNodes) > 0 || len(featureChain.OfficialNodes) > 0 {
		return "manifest+gox-skeleton"
	}
	return "manifest-only"
}

func featureStatus(featureChain model.FeatureChain, findings []model.SemanticDiff) string {
	if len(findings) > 0 {
		return "drifted"
	}
	if countExtractedNodes(featureChain.LocalNodes)+countExtractedNodes(featureChain.OfficialNodes) > 0 {
		return "aligned"
	}
	return "placeholder"
}

func countExtractedNodes(nodes []model.ChainNode) int {
	count := 0
	for _, node := range nodes {
		if metadataBool(node.Metadata, "extracted") {
			count++
		}
	}
	return count
}

func metadataBool(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}

	value, ok := metadata[key]
	if !ok {
		return false
	}

	booleanValue, ok := value.(bool)
	return ok && booleanValue
}
