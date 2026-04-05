package manifest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadAndValidateExampleManifest(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "examples", "sub2api.manifest.example.yaml"))

	loaded, result, err := LoadAndValidate(path)
	require.NoError(t, err)
	require.True(t, result.Valid)
	require.Equal(t, "sub2api", loaded.RepoKind)
	require.Len(t, loaded.Features, 7)
}

func TestValidateInvalidManifest(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "testdata", "manifest", "invalid.yaml"))

	_, result, err := LoadAndValidate(path)
	require.Error(t, err)
	require.False(t, result.Valid)
	require.NotEmpty(t, result.Errors)
	require.Contains(t, strings.Join(result.Errors, "\n"), "duplicate feature id")
}
