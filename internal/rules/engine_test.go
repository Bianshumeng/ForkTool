package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"forktool/internal/adapters/gox"
	"forktool/internal/decision"
	"forktool/internal/discovery"
	"forktool/internal/ir"
	"forktool/internal/manifest"
	"forktool/pkg/model"
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

func TestEngineApplySupportsAdditionalRules(t *testing.T) {
	type ruleCase struct {
		name             string
		ruleID           string
		localContent     string
		officialContent  string
		expectedCategory string
	}

	testCases := []ruleCase{
		{
			name:             "claude metadata userid format",
			ruleID:           "claude-metadata-userid-format",
			localContent:     "func buildOAuthMetadataUserID() string { return \"legacy\" }",
			officialContent:  "func buildOAuthMetadataUserID() string { return FormatMetadataUserID(\"device\", \"account\", \"session\", \"2.1.78\") }",
			expectedCategory: "metadata-format",
		},
		{
			name:             "response header filter",
			ruleID:           "response-header-filter",
			localContent:     "func writeHeaders() { dst.Set(\"x-test\", \"1\") }",
			officialContent:  "func writeHeaders() { if s.responseHeaderFilter != nil { responseheaders.WriteFilteredHeaders(dst, src, s.responseHeaderFilter) } }",
			expectedCategory: "response-header",
		},
		{
			name:             "openai originator compatibility",
			ruleID:           "openai-originator-compatibility",
			localContent:     "func buildHeaders() { req.Header.Set(\"Authorization\", \"Bearer test\") }",
			officialContent:  "func buildHeaders() { if req.Header.Get(\"OpenAI-Beta\") == \"\" { req.Header.Set(\"OpenAI-Beta\", \"responses=experimental\") }; req.Header.Set(\"originator\", \"codex_cli_rs\"); _ = openai.IsCodexOfficialClientByHeaders(\"ua\", \"originator\") }",
			expectedCategory: "request-header",
		},
		{
			name:             "openai ws previous response id",
			ruleID:           "openai-ws-previous-response-id",
			localContent:     "func ws() { payload := []byte(`{\"type\":\"response.create\"}`) }",
			officialContent:  "func ws() { previous_response_id := \"resp_1\"; _ = previous_response_id; action := \"drop_previous_response_id_retry\"; _ = action }",
			expectedCategory: "session-routing",
		},
		{
			name:             "openai ws turn metadata replay",
			ruleID:           "openai-ws-turn-metadata-replay",
			localContent:     "func wsReplay() { return }",
			officialContent:  "func wsReplay() { has_turn_metadata := true; _, _, _ = buildOpenAIWSReplayInputSequence(has_turn_metadata) }",
			expectedCategory: "request-body",
		},
		{
			name:             "observability upstream model",
			ruleID:           "observability-upstream-model",
			localContent:     "type ForwardResult struct{}",
			officialContent:  "type ForwardResult struct { UpstreamModel string }",
			expectedCategory: "observability",
		},
		{
			name:             "gemini failover semantics",
			ruleID:           "gemini-failover-semantics",
			localContent:     "func GeminiV1BetaModels() { handleGeminiFailoverExhausted(nil, nil) }",
			officialContent:  "func GeminiV1BetaModels() { failoverAction := fs.HandleFailoverError(ctx, svc, account.ID, account.Platform, failoverErr); _ = failoverAction; handleGeminiFailoverExhausted(c, failoverErr) }",
			expectedCategory: "error-semantics",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			localRepo := t.TempDir()
			officialRepo := t.TempDir()
			relativeFile := filepath.ToSlash("backend/internal/service/rule_case.go")

			writeRuleCaseFile(t, localRepo, relativeFile, testCase.localContent)
			writeRuleCaseFile(t, officialRepo, relativeFile, testCase.officialContent)

			feature := model.FeatureChain{
				ID:              "rule-case",
				Name:            testCase.name,
				RiskLevel:       "critical",
				SemanticRules:   []string{testCase.ruleID},
				DefaultDecision: "official-required",
				LocalNodes: []model.ChainNode{
					{
						Kind:     model.NodeKindService,
						Language: "go",
						FilePath: relativeFile,
						Metadata: map[string]any{"extracted": true},
					},
				},
				OfficialNodes: []model.ChainNode{
					{
						Kind:     model.NodeKindService,
						Language: "go",
						FilePath: relativeFile,
						Metadata: map[string]any{"extracted": true},
					},
				},
			}

			findings, err := NewEngine(localRepo, officialRepo).Apply(context.Background(), feature, nil)
			require.NoError(t, err)
			require.Len(t, findings, 1)
			require.Equal(t, testCase.expectedCategory, findings[0].Category)
		})
	}
}

func TestBuildEvidencePrefersSourceFilesOverTests(t *testing.T) {
	feature := model.FeatureChain{
		ID: "evidence-pref",
		LocalNodes: []model.ChainNode{
			{Kind: model.NodeKindService, FilePath: "backend/internal/service/openai_gateway_service.go"},
			{Kind: model.NodeKindTest, FilePath: "backend/internal/service/openai_gateway_service_test.go"},
		},
		OfficialNodes: []model.ChainNode{
			{Kind: model.NodeKindService, FilePath: "backend/internal/service/openai_gateway_service.go"},
			{Kind: model.NodeKindTest, FilePath: "backend/internal/service/openai_gateway_service_test.go"},
		},
	}

	input := ruleInput{
		localContents: map[string]string{
			"backend/internal/service/openai_gateway_service.go":      "normalizeOpenAIPassthroughOAuthBody",
			"backend/internal/service/openai_gateway_service_test.go": "normalizeOpenAIPassthroughOAuthBody",
		},
		officialContents: map[string]string{
			"backend/internal/service/openai_gateway_service.go":      "normalizeOpenAIPassthroughOAuthBody",
			"backend/internal/service/openai_gateway_service_test.go": "normalizeOpenAIPassthroughOAuthBody",
		},
	}

	evidence := buildEvidence(feature, input, []string{"normalizeOpenAIPassthroughOAuthBody"}, []string{"normalizeOpenAIPassthroughOAuthBody"})
	require.Len(t, evidence, 2)
	require.Equal(t, "backend/internal/service/openai_gateway_service.go", evidence[0].FilePath)
	require.Equal(t, "backend/internal/service/openai_gateway_service.go", evidence[1].FilePath)
}

func writeRuleCaseFile(t *testing.T, repoRoot, relativePath, content string) {
	t.Helper()

	absolutePath := filepath.Join(repoRoot, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, []byte("package service\n\n"+content+"\n"), 0o644))
}

func projectRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}
