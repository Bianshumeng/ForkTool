package decision

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDecisionFile(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "testdata", "audit", "decisions.yaml"))

	loaded, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, 1, loaded.Version)
	require.Len(t, loaded.Decisions, 2)
	require.Equal(t, "test-missing", loaded.Decisions[0].Decision)
}

func TestFilterForFeature(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "testdata", "audit", "decisions.yaml"))

	loaded, err := Load(path)
	require.NoError(t, err)

	filtered := FilterForFeature(loaded, "openai-responses-compact")
	require.Len(t, filtered, 1)
	require.Equal(t, "test-file-presence", filtered[0].Scope)
}
