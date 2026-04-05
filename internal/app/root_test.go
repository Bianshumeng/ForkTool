package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCommand(t *testing.T) {
	stdout, _, err := executeCommand(t, "", "version")
	require.NoError(t, err)
	require.Contains(t, stdout, "forktool "+Version)
}

func TestInitCommandCreatesWorkspace(t *testing.T) {
	workdir := t.TempDir()

	stdout, _, err := executeCommand(t, workdir, "init", "--repo-kind", "sub2api")
	require.NoError(t, err)
	require.Contains(t, stdout, "configPath")
	require.FileExists(t, filepath.Join(workdir, ".forktool", "config.yaml"))
	require.FileExists(t, filepath.Join(workdir, "decisions", "sub2api.local-decisions.yaml"))
	require.FileExists(t, filepath.Join(workdir, "manifests", "sub2api.yaml"))
	require.Contains(t, stdout, `"manifestPath": "./manifests/sub2api.yaml"`)
}

func TestManifestValidateCommand(t *testing.T) {
	manifestPath := filepath.Join(projectRoot(t), "testdata", "manifest", "valid.yaml")

	stdout, _, err := executeCommand(t, "", "manifest", "validate", "--path", manifestPath)
	require.NoError(t, err)
	require.Contains(t, stdout, `"valid": true`)
}

func TestDoctorCommandReportsWorkspaceReadiness(t *testing.T) {
	workdir := t.TempDir()

	_, _, initErr := executeCommand(t, workdir, "init", "--repo-kind", "sub2api")
	require.NoError(t, initErr)

	stdout, _, err := executeCommand(t, workdir, "doctor")
	require.NoError(t, err)
	require.Contains(t, stdout, `"configFound": true`)
	require.Contains(t, stdout, `"name": "decision-file"`)
	require.Contains(t, stdout, `"name": "baseline-inputs"`)
}

func TestManifestListCommand(t *testing.T) {
	manifestPath := filepath.Join(projectRoot(t), "testdata", "audit", "manifest.yaml")

	stdout, _, err := executeCommand(t, "", "manifest", "list", "--path", manifestPath)
	require.NoError(t, err)
	require.Contains(t, stdout, `"featureCount": 5`)
	require.Contains(t, stdout, `"id": "claude-count-tokens"`)
}

func TestScanFeatureCommandWritesPlaceholderReport(t *testing.T) {
	workdir := t.TempDir()
	manifestPath := filepath.Join(projectRoot(t), "examples", "sub2api.manifest.example.yaml")
	outputDir := filepath.Join(workdir, "out")

	stdout, _, err := executeCommand(t, workdir,
		"scan", "feature",
		"--feature", "claude-count-tokens",
		"--manifest", manifestPath,
		"--format", "json",
		"--out", outputDir,
	)
	require.NoError(t, err)
	require.Contains(t, stdout, `"featureId": "claude-count-tokens"`)
	require.FileExists(t, filepath.Join(outputDir, "context.json"))
	require.FileExists(t, filepath.Join(outputDir, "report.json"))
}

func TestScanFeatureCommandUsesGoASTDiscovery(t *testing.T) {
	workdir := t.TempDir()
	manifestPath := filepath.Join(projectRoot(t), "testdata", "gox", "manifest.yaml")
	repoPath := filepath.Join(projectRoot(t), "testdata", "gox", "repo")
	outputDir := filepath.Join(workdir, "out")

	stdout, _, err := executeCommand(t, workdir,
		"scan", "feature",
		"--feature", "claude-count-tokens",
		"--manifest", manifestPath,
		"--local", repoPath,
		"--official", repoPath,
		"--format", "json",
		"--out", outputDir,
	)
	require.NoError(t, err)
	require.Contains(t, stdout, `"discoveryMode": "gox-ast"`)
	require.Contains(t, stdout, `"localNodeCount": 6`)
	require.Contains(t, stdout, `"officialNodeCount": 6`)

	reportContent, readErr := os.ReadFile(filepath.Join(outputDir, "report.json"))
	require.NoError(t, readErr)
	require.Contains(t, string(reportContent), `"status": "aligned"`)
	require.Contains(t, string(reportContent), `"source": "go-ast"`)
	require.Contains(t, string(reportContent), `"source": "go-ast-derived-call"`)
}

func TestScanFeatureCommandReturnsFindingsForAuditFixture(t *testing.T) {
	workdir := t.TempDir()
	root := projectRoot(t)
	manifestPath := filepath.Join(root, "testdata", "audit", "manifest.yaml")
	localPath := filepath.Join(root, "testdata", "audit", "local")
	officialPath := filepath.Join(root, "testdata", "audit", "official")
	decisionPath := filepath.Join(root, "testdata", "audit", "decisions.yaml")
	outputDir := filepath.Join(workdir, "audit-feature")

	stdout, _, err := executeCommand(t, workdir,
		"scan", "feature",
		"--feature", "openai-responses-compact",
		"--manifest", manifestPath,
		"--decision-file", decisionPath,
		"--local", localPath,
		"--official", officialPath,
		"--format", "json",
		"--out", outputDir,
	)
	require.Error(t, err)
	require.Equal(t, ExitFindings, ExitCode(err))
	require.Contains(t, stdout, `"findingCount": 4`)

	reportContent, readErr := os.ReadFile(filepath.Join(outputDir, "report.json"))
	require.NoError(t, readErr)
	require.Contains(t, string(reportContent), `"status": "drifted"`)
	require.Contains(t, string(reportContent), `"decisionTag": "test-missing"`)
	require.Contains(t, string(reportContent), `"title": "compact 请求未透传 /compact suffix"`)
}

func TestScanReleaseCommandAggregatesFindings(t *testing.T) {
	workdir := t.TempDir()
	root := projectRoot(t)
	manifestPath := filepath.Join(root, "testdata", "audit", "manifest.yaml")
	localPath := filepath.Join(root, "testdata", "audit", "local")
	officialPath := filepath.Join(root, "testdata", "audit", "official")
	decisionPath := filepath.Join(root, "testdata", "audit", "decisions.yaml")
	outputDir := filepath.Join(workdir, "audit-release")

	stdout, _, err := executeCommand(t, workdir,
		"scan", "release",
		"--manifest", manifestPath,
		"--decision-file", decisionPath,
		"--local", localPath,
		"--official", officialPath,
		"--critical-only",
		"--format", "json",
		"--out", outputDir,
	)
	require.Error(t, err)
	require.Equal(t, ExitFindings, ExitCode(err))
	require.Contains(t, stdout, `"featuresScanned": 3`)
	require.Contains(t, stdout, `"findingCount": 7`)

	reportContent, readErr := os.ReadFile(filepath.Join(outputDir, "report.json"))
	require.NoError(t, readErr)
	require.Contains(t, string(reportContent), `"criticalFindings": 2`)
	require.Contains(t, string(reportContent), `"highFindings": 5`)
}

func TestScanFeatureCommandDiscoversMultiStackEvidence(t *testing.T) {
	workdir := t.TempDir()
	root := projectRoot(t)
	manifestPath := filepath.Join(root, "testdata", "multistack", "manifest.yaml")
	localPath := filepath.Join(root, "testdata", "multistack", "local")
	officialPath := filepath.Join(root, "testdata", "multistack", "official")
	outputDir := filepath.Join(workdir, "multistack")

	stdout, _, err := executeCommand(t, workdir,
		"scan", "feature",
		"--feature", "status-page-settings",
		"--manifest", manifestPath,
		"--local", localPath,
		"--official", officialPath,
		"--format", "json",
		"--out", outputDir,
	)
	require.Error(t, err)
	require.Equal(t, ExitFindings, ExitCode(err))
	require.Contains(t, stdout, `"discoveryMode": "configx+gox+markdownx+sqlx+tsvue"`)

	reportContent, readErr := os.ReadFile(filepath.Join(outputDir, "report.json"))
	require.NoError(t, readErr)
	require.Contains(t, string(reportContent), `"category": "config-chain"`)
	require.Contains(t, string(reportContent), `"category": "migration-chain"`)
	require.Contains(t, string(reportContent), `"category": "frontend-parity"`)
	require.Contains(t, string(reportContent), `"category": "doc-coverage"`)
	require.Contains(t, string(reportContent), `"source": "text-config-scan"`)
	require.Contains(t, string(reportContent), `"source": "text-markdown-scan"`)
	require.Contains(t, string(reportContent), `"source": "text-sql-scan"`)
	require.Contains(t, string(reportContent), `"source": "text-frontend-scan"`)
}

func TestReportRenderCommandWritesMarkdown(t *testing.T) {
	workdir := t.TempDir()
	manifestPath := filepath.Join(projectRoot(t), "testdata", "gox", "manifest.yaml")
	repoPath := filepath.Join(projectRoot(t), "testdata", "gox", "repo")
	scanOutputDir := filepath.Join(workdir, "scan-out")
	renderOutputPath := filepath.Join(workdir, "rendered.md")

	_, _, scanErr := executeCommand(t, workdir,
		"scan", "feature",
		"--feature", "claude-count-tokens",
		"--manifest", manifestPath,
		"--local", repoPath,
		"--official", repoPath,
		"--format", "json",
		"--out", scanOutputDir,
	)
	require.NoError(t, scanErr)

	stdout, _, renderErr := executeCommand(t, workdir,
		"report", "render",
		"--input", filepath.Join(scanOutputDir, "report.json"),
		"--format", "md",
		"--out", renderOutputPath,
	)
	require.NoError(t, renderErr)
	require.Empty(t, stdout)

	renderedContent, readErr := os.ReadFile(renderOutputPath)
	require.NoError(t, readErr)
	require.Contains(t, string(renderedContent), "# ForkTool Audit Report")
	require.Contains(t, string(renderedContent), "Feature: claude-count-tokens")
}

func TestReportRenderCommandWritesBDDraft(t *testing.T) {
	workdir := t.TempDir()
	root := projectRoot(t)
	manifestPath := filepath.Join(root, "testdata", "audit", "manifest.yaml")
	localPath := filepath.Join(root, "testdata", "audit", "local")
	officialPath := filepath.Join(root, "testdata", "audit", "official")
	outputDir := filepath.Join(workdir, "audit-feature")
	renderOutputPath := filepath.Join(workdir, "report.bd.md")

	_, _, scanErr := executeCommand(t, workdir,
		"scan", "feature",
		"--feature", "openai-responses-compact",
		"--manifest", manifestPath,
		"--local", localPath,
		"--official", officialPath,
		"--format", "json",
		"--out", outputDir,
	)
	require.Error(t, scanErr)
	require.Equal(t, ExitFindings, ExitCode(scanErr))

	stdout, _, renderErr := executeCommand(t, workdir,
		"report", "render",
		"--input", filepath.Join(outputDir, "report.json"),
		"--format", "bd",
		"--out", renderOutputPath,
	)
	require.NoError(t, renderErr)
	require.Empty(t, stdout)

	renderedContent, readErr := os.ReadFile(renderOutputPath)
	require.NoError(t, readErr)
	require.Contains(t, string(renderedContent), "# ForkTool BD Draft")
	require.Contains(t, string(renderedContent), "Suggested Issues")
	require.Contains(t, string(renderedContent), "Suggested Title")
}

func TestSplitSemanticRules(t *testing.T) {
	supported, unsupported := splitSemanticRules([]string{
		"claude-count-tokens-beta-suffix",
		"response-header-filter",
		"openai-session-isolation",
	})

	require.Equal(t, []string{
		"claude-count-tokens-beta-suffix",
		"response-header-filter",
		"openai-session-isolation",
	}, supported)
	require.Empty(t, unsupported)
}

func executeCommand(t *testing.T, workdir string, args ...string) (string, string, error) {
	t.Helper()

	command := NewRootCommand()
	command.SetArgs(args)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if strings.TrimSpace(workdir) != "" {
		restoreDir := chdirForTest(t, workdir)
		defer restoreDir()
	}

	err := command.Execute()
	return stdout.String(), stderr.String(), err
}

func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()

	original, err := filepath.Abs(".")
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))

	return func() {
		require.NoError(t, os.Chdir(original))
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}
