package gox

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"forktool/internal/discovery"
	"forktool/internal/manifest"
	"forktool/pkg/model"
)

func TestDiscoverExtractsRoutesSymbolsAndTestsFromSource(t *testing.T) {
	repoRoot := filepath.Join(projectRoot(t), "testdata", "gox", "repo")
	manifestPath := filepath.Join(projectRoot(t), "testdata", "gox", "manifest.yaml")

	loadedManifest, result, err := manifest.LoadAndValidate(manifestPath)
	require.NoError(t, err)
	require.True(t, result.Valid)

	feature := loadedManifest.Features[0]
	nodes, err := Adapter{}.Discover(context.Background(), discovery.DiscoverRequest{
		RepoRoot: repoRoot,
		Feature:  feature,
		RepoSide: "local",
	})
	require.NoError(t, err)
	require.NotEmpty(t, nodes)

	require.Contains(t, filterNodeKeys(nodes, model.NodeKindRoute), "backend/internal/server/routes/gateway.go#RegisterGatewayRoutes")
	require.Contains(t, filterNodeKeys(nodes, model.NodeKindService), "backend/internal/service/gateway_service.go#ForwardCountTokens")
	require.Contains(t, filterNodeKeys(nodes, model.NodeKindService), "backend/internal/service/gateway_service.go#buildCountTokensRequest")
	require.Contains(t, filterNodeKeys(nodes, model.NodeKindTest), "backend/internal/service/gateway_service_test.go#TestForwardCountTokens")

	var routeChecked bool
	for _, node := range nodes {
		if node.Kind == model.NodeKindRoute && node.Metadata["matchedFullPath"] == "/v1/messages/count_tokens" {
			routeChecked = true
			require.Equal(t, "POST", node.Metadata["method"])
			require.Equal(t, "h.Gateway.CountTokens", node.Metadata["primaryHandler"])
			require.Len(t, node.Relations, 1)
			require.Equal(t, "h.Gateway.CountTokens", node.Relations[0].Target)
		}
		if node.SymbolName == "ForwardCountTokens" {
			require.Greater(t, node.Range.StartLine, 0)
			require.True(t, metadataBool(node.Metadata, "extracted"))
			require.Equal(t, "go-ast", node.Metadata["source"])
		}
	}
	require.True(t, routeChecked)
}

func TestDiscoverMatchesWildcardRoutesWithGroupPrefix(t *testing.T) {
	repoRoot := filepath.Join(projectRoot(t), "testdata", "gox", "repo")
	feature := model.ManifestFeature{
		ID:        "openai-responses-compact",
		Name:      "OpenAI compact route",
		RiskLevel: "critical",
		Languages: []string{"go"},
		Chain: model.ManifestChain{
			Routes: []model.ManifestRoute{
				{PathPattern: "/v1/responses/compact"},
			},
		},
		SemanticRules: []string{"openai-compact-path-suffix"},
		Decisions: model.ManifestDecisions{
			Default: "official-required",
		},
	}

	nodes, err := Adapter{}.Discover(context.Background(), discovery.DiscoverRequest{
		RepoRoot: repoRoot,
		Feature:  feature,
		RepoSide: "official",
	})
	require.NoError(t, err)
	require.Len(t, nodes, 1)

	routeNode := nodes[0]
	require.Equal(t, model.NodeKindRoute, routeNode.Kind)
	require.Equal(t, "/v1/responses/*subpath", routeNode.Metadata["matchedFullPath"])
	require.Equal(t, "h.OpenAIGateway.Responses", routeNode.Metadata["primaryHandler"])
	require.Contains(t, routeNode.Metadata["handlerTargets"], "h.OpenAIGateway.Responses")
}

func TestDiscoverFallsBackWhenSymbolOrTestIsMissing(t *testing.T) {
	repoRoot := filepath.Join(projectRoot(t), "testdata", "gox", "repo")
	feature := model.ManifestFeature{
		ID:        "missing",
		Name:      "Missing nodes",
		RiskLevel: "high",
		Languages: []string{"go"},
		Chain: model.ManifestChain{
			Routes: []model.ManifestRoute{
				{PathPattern: "/v1/not-found"},
			},
			Symbols: []model.ManifestSymbol{
				{
					File:      "backend/internal/service/gateway_service.go",
					Functions: []string{"MissingFunction"},
				},
			},
			Tests: []string{"backend/internal/service/missing_test.go"},
		},
		SemanticRules: []string{"placeholder-rule"},
		Decisions: model.ManifestDecisions{
			Default: "official-required",
		},
	}

	nodes, err := Adapter{}.Discover(context.Background(), discovery.DiscoverRequest{
		RepoRoot: repoRoot,
		Feature:  feature,
		RepoSide: "official",
	})
	require.NoError(t, err)
	require.Len(t, nodes, 3)

	for _, node := range nodes {
		require.False(t, metadataBool(node.Metadata, "extracted"))
	}
}

func filterNodeKeys(nodes []model.ChainNode, kind model.NodeKind) []string {
	keys := make([]string, 0)
	for _, node := range nodes {
		if node.Kind != kind {
			continue
		}
		keys = append(keys, node.FilePath+"#"+node.SymbolName)
	}
	return keys
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

func projectRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	return root
}
