package rules

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"forktool/internal/adapters/gox"
	"forktool/internal/decision"
	"forktool/internal/discovery"
	"forktool/internal/ir"
	"forktool/internal/manifest"
)

func TestEngineApplyProducesDeterministicFindings(t *testing.T) {
	root := projectRoot(t)
	manifestPath := filepath.Join(root, "testdata", "audit", "manifest.yaml")
	localRepo := filepath.Join(root, "testdata", "audit", "local")
	officialRepo := filepath.Join(root, "testdata", "audit", "official")
	decisionPath := filepath.Join(root, "testdata", "audit", "decisions.yaml")

	loadedManifest, result, err := manifest.LoadAndValidate(manifestPath)
	require.NoError(t, err)
	require.True(t, result.Valid)

	decisionFile, err := decision.Load(decisionPath)
	require.NoError(t, err)

	manager := discovery.NewManager(gox.New())

	for _, feature := range loadedManifest.Features {
		localNodes, err := manager.Discover(context.Background(), discovery.DiscoverRequest{
			RepoRoot: localRepo,
			Feature:  feature,
			RepoSide: "local",
		})
		require.NoError(t, err)

		officialNodes, err := manager.Discover(context.Background(), discovery.DiscoverRequest{
			RepoRoot: officialRepo,
			Feature:  feature,
			RepoSide: "official",
		})
		require.NoError(t, err)

		featureChain := ir.NewFeatureChain(feature, localNodes, officialNodes, decision.FilterForFeature(decisionFile, feature.ID))
		findings, err := NewEngine(localRepo, officialRepo).Apply(context.Background(), featureChain, featureChain.DecisionHints)
		require.NoError(t, err)

		switch feature.ID {
		case "claude-count-tokens":
			require.Len(t, findings, 1)
			require.Equal(t, "request-url", findings[0].Category)
			require.Equal(t, "critical", findings[0].Severity)
		case "claude-messages-mainchain":
			require.Len(t, findings, 2)
		case "openai-responses-compact":
			require.Len(t, findings, 4)
			require.Equal(t, "test-missing", findings[3].DecisionTag)
		case "gemini-messages-compat":
			require.Len(t, findings, 1)
			require.Equal(t, "observability", findings[0].Category)
		case "gemini-native-v1beta":
			require.Len(t, findings, 1)
			require.Equal(t, "session-routing", findings[0].Category)
		}
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}
