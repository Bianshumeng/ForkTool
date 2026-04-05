package baseline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"forktool/pkg/model"
)

func TestVerifySuccess(t *testing.T) {
	repoPath, commit := createGitRepo(t, "https://example.com/sub2api.git")

	result, err := Verify(context.Background(), VerifyInput{
		Official: model.RepoConfig{
			Path:      repoPath,
			Kind:      "official",
			RemoteURL: "https://example.com/sub2api.git",
			Tag:       "v0.1.105",
			Commit:    commit,
		},
		RemoteName: "origin",
	})
	require.NoError(t, err)
	require.True(t, result.Valid)
	require.Equal(t, commit, result.ResolvedCommit)
	require.Equal(t, commit, result.ResolvedTagCommit)
}

func TestVerifyRemoteMismatch(t *testing.T) {
	repoPath, _ := createGitRepo(t, "https://example.com/sub2api.git")

	result, err := Verify(context.Background(), VerifyInput{
		Official: model.RepoConfig{
			Path:      repoPath,
			Kind:      "official",
			RemoteURL: "https://example.com/wrong.git",
			Tag:       "v0.1.105",
		},
		RemoteName: "origin",
	})
	require.NoError(t, err)
	require.False(t, result.Valid)
	require.Contains(t, strings.Join(result.Errors, "\n"), "remote URL mismatch")
}

func createGitRepo(t *testing.T, remoteURL string) (string, string) {
	t.Helper()

	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.name", "ForkTool Test")
	runGit(t, repoPath, "config", "user.email", "forktool@example.com")

	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("baseline fixture\n"), 0o644))
	runGit(t, repoPath, "add", "README.md")
	runGit(t, repoPath, "commit", "-m", "initial commit")
	runGit(t, repoPath, "remote", "add", "origin", remoteURL)
	runGit(t, repoPath, "tag", "v0.1.105")

	commit := runGit(t, repoPath, "rev-parse", "HEAD")
	return repoPath, strings.TrimSpace(commit)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(output))
	return string(output)
}
