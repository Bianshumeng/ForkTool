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
}

func TestManifestValidateCommand(t *testing.T) {
	manifestPath := filepath.Join(projectRoot(t), "testdata", "manifest", "valid.yaml")

	stdout, _, err := executeCommand(t, "", "manifest", "validate", "--path", manifestPath)
	require.NoError(t, err)
	require.Contains(t, stdout, `"valid": true`)
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
