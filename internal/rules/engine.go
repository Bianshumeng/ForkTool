package rules

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"forktool/pkg/model"
)

type RuleEngine interface {
	Apply(ctx context.Context, feature model.FeatureChain, decisions []model.DecisionHint) ([]model.SemanticDiff, error)
}

type Engine struct {
	localRepoRoot    string
	officialRepoRoot string
}

func NewEngine(localRepoRoot, officialRepoRoot string) RuleEngine {
	return Engine{
		localRepoRoot:    localRepoRoot,
		officialRepoRoot: officialRepoRoot,
	}
}

func SupportedRuleIDs() []string {
	return []string{
		"claude-metadata-userid-format",
		"claude-count-tokens-beta-suffix",
		"claude-session-hash-normalization",
		"http-header-wire-casing",
		"response-header-filter",
		"openai-compact-path-suffix",
		"openai-originator-compatibility",
		"openai-session-isolation",
		"openai-passthrough-body-normalization",
		"openai-ws-previous-response-id",
		"openai-ws-turn-metadata-replay",
		"observability-upstream-model",
		"gemini-failover-semantics",
		"gemini-upstream-model-preserved",
		"gemini-digest-prefix-ua-normalization",
		"test-file-presence",
	}
}

func (e Engine) Apply(_ context.Context, feature model.FeatureChain, decisions []model.DecisionHint) ([]model.SemanticDiff, error) {
	input, err := e.buildInput(feature)
	if err != nil {
		return nil, err
	}

	findings := make([]model.SemanticDiff, 0)
	for _, ruleID := range feature.SemanticRules {
		switch ruleID {
		case "claude-metadata-userid-format":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "metadata-format",
				titleMissing:       "metadata.user_id 未使用官方格式化逻辑",
				titleUnexpected:    "metadata.user_id 仍保留官方已移除的格式化逻辑",
				descriptionMissing: "官方链路命中 FormatMetadataUserID，本地未命中。",
				descriptionExtra:   "本地命中 FormatMetadataUserID，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"FormatMetadataUserID"},
				officialTokens:     []string{"FormatMetadataUserID"},
			}))
		case "claude-count-tokens-beta-suffix":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "critical",
				category:           "request-url",
				titleMissing:       "count_tokens 上游 URL 丢失 beta=true",
				titleUnexpected:    "count_tokens 本地仍保留了官方已移除的 beta=true",
				descriptionMissing: "官方实现包含 ?beta=true，但本地未命中这一语义。",
				descriptionExtra:   "本地仍命中 ?beta=true，但官方基线已不包含这一语义。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"?beta=true"},
				officialTokens:     []string{"?beta=true"},
			}))
		case "claude-session-hash-normalization":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "session-routing",
				titleMissing:       "session hash 未使用官方 UA normalize",
				titleUnexpected:    "session hash 仍保留官方已移除的 UA normalize 语义",
				descriptionMissing: "官方链路依赖 NormalizeSessionUserAgent，本地未命中。",
				descriptionExtra:   "本地命中 NormalizeSessionUserAgent，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"NormalizeSessionUserAgent"},
				officialTokens:     []string{"NormalizeSessionUserAgent"},
			}))
		case "http-header-wire-casing":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "request-header",
				titleMissing:       "请求头透传未保持官方 wire casing/raw write",
				titleUnexpected:    "本地请求头透传保留了官方已移除的 wire casing/raw write 逻辑",
				descriptionMissing: "官方链路命中 resolveWireCasing + addHeaderRaw，本地未完整命中。",
				descriptionExtra:   "本地命中 resolveWireCasing + addHeaderRaw，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"resolveWireCasing", "addHeaderRaw"},
				officialTokens:     []string{"resolveWireCasing", "addHeaderRaw"},
				localFallbackAny:   []string{"Header.Add(", "Header.Set("},
			}))
		case "response-header-filter":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "response-header",
				titleMissing:       "响应头过滤链未对齐官方",
				titleUnexpected:    "本地保留了官方已移除的响应头过滤链",
				descriptionMissing: "官方链路命中 responseHeaderFilter + responseheaders.WriteFilteredHeaders，本地未完整命中。",
				descriptionExtra:   "本地命中 responseHeaderFilter + responseheaders.WriteFilteredHeaders，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"responseHeaderFilter", "responseheaders.WriteFilteredHeaders"},
				officialTokens:     []string{"responseHeaderFilter", "responseheaders.WriteFilteredHeaders"},
			}))
		case "openai-compact-path-suffix":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "critical",
				category:           "request-url",
				titleMissing:       "compact 请求未透传 /compact suffix",
				titleUnexpected:    "本地保留了官方已移除的 /compact suffix",
				descriptionMissing: "官方链路命中 /responses/compact，本地未命中。",
				descriptionExtra:   "本地命中 /responses/compact，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"/responses/compact"},
				officialTokens:     []string{"/responses/compact"},
				useRouteNodes:      true,
			}))
		case "openai-originator-compatibility":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "request-header",
				titleMissing:       "OpenAI originator / beta 兼容头未按官方保留",
				titleUnexpected:    "本地保留了官方已移除的 OpenAI originator / beta 兼容逻辑",
				descriptionMissing: "官方链路命中 OpenAI-Beta + originator + IsCodexOfficialClientByHeaders，本地未完整命中。",
				descriptionExtra:   "本地命中 OpenAI-Beta + originator + IsCodexOfficialClientByHeaders，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"OpenAI-Beta", "originator", "IsCodexOfficialClientByHeaders"},
				officialTokens:     []string{"OpenAI-Beta", "originator", "IsCodexOfficialClientByHeaders"},
			}))
		case "openai-session-isolation":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "session-routing",
				titleMissing:       "OpenAI session / conversation 隔离逻辑缺失",
				titleUnexpected:    "本地保留了官方已移除的 OpenAI session isolation 逻辑",
				descriptionMissing: "官方链路命中 isolateOpenAISessionID，本地未命中。",
				descriptionExtra:   "本地命中 isolateOpenAISessionID，但官方基线未命中。",
				recommendedAction:  "merge-official-into-local-hook",
				localTokens:        []string{"isolateOpenAISessionID"},
				officialTokens:     []string{"isolateOpenAISessionID"},
			}))
		case "openai-passthrough-body-normalization":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "request-body",
				titleMissing:       "OpenAI passthrough body normalize 与官方不一致",
				titleUnexpected:    "本地保留了官方已移除的 passthrough body normalize 逻辑",
				descriptionMissing: "官方链路命中 normalizeOpenAIPassthroughOAuthBody，本地未命中。",
				descriptionExtra:   "本地命中 normalizeOpenAIPassthroughOAuthBody，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"normalizeOpenAIPassthroughOAuthBody"},
				officialTokens:     []string{"normalizeOpenAIPassthroughOAuthBody"},
			}))
		case "openai-ws-previous-response-id":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "session-routing",
				titleMissing:       "WS previous_response_id 恢复链未按官方保留",
				titleUnexpected:    "本地保留了官方已移除的 WS previous_response_id 恢复链",
				descriptionMissing: "官方链路命中 previous_response_id + drop_previous_response_id 重试逻辑，本地未完整命中。",
				descriptionExtra:   "本地命中 previous_response_id + drop_previous_response_id 重试逻辑，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"previous_response_id", "drop_previous_response_id"},
				officialTokens:     []string{"previous_response_id", "drop_previous_response_id"},
			}))
		case "openai-ws-turn-metadata-replay":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "request-body",
				titleMissing:       "WS turn metadata replay 链未按官方保留",
				titleUnexpected:    "本地保留了官方已移除的 WS turn metadata replay 链",
				descriptionMissing: "官方链路命中 has_turn_metadata + buildOpenAIWSReplayInputSequence，本地未完整命中。",
				descriptionExtra:   "本地命中 has_turn_metadata + buildOpenAIWSReplayInputSequence，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"has_turn_metadata", "buildOpenAIWSReplayInputSequence"},
				officialTokens:     []string{"has_turn_metadata", "buildOpenAIWSReplayInputSequence"},
			}))
		case "observability-upstream-model":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "observability",
				titleMissing:       "upstream_model 观测链缺失",
				titleUnexpected:    "本地保留了官方已移除的 upstream_model 观测链",
				descriptionMissing: "官方链路命中 UpstreamModel，本地未命中。",
				descriptionExtra:   "本地命中 UpstreamModel，但官方基线未命中。",
				recommendedAction:  "verify-observability-chain",
				localTokens:        []string{"UpstreamModel"},
				officialTokens:     []string{"UpstreamModel"},
			}))
		case "gemini-failover-semantics":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "error-semantics",
				titleMissing:       "Gemini failover 语义未按官方处理",
				titleUnexpected:    "本地保留了官方已移除的 Gemini failover 处理链",
				descriptionMissing: "官方链路命中 HandleFailoverError + handleGeminiFailoverExhausted，本地未完整命中。",
				descriptionExtra:   "本地命中 HandleFailoverError + handleGeminiFailoverExhausted，但官方基线未命中。",
				recommendedAction:  "merge-official-into-local-hook",
				localTokens:        []string{"HandleFailoverError", "handleGeminiFailoverExhausted"},
				officialTokens:     []string{"HandleFailoverError", "handleGeminiFailoverExhausted"},
			}))
		case "gemini-upstream-model-preserved":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "observability",
				titleMissing:       "ForwardResult.UpstreamModel 被本地删除",
				titleUnexpected:    "本地保留了官方已移除的 UpstreamModel 观测链",
				descriptionMissing: "官方链路命中 UpstreamModel，本地未命中。",
				descriptionExtra:   "本地命中 UpstreamModel，但官方基线未命中。",
				recommendedAction:  "verify-observability-chain",
				localTokens:        []string{"UpstreamModel"},
				officialTokens:     []string{"UpstreamModel"},
			}))
		case "gemini-digest-prefix-ua-normalization":
			appendFinding(&findings, comparePresence(ruleID, feature, decisions, input, ruleDefinition{
				severity:           "high",
				category:           "session-routing",
				titleMissing:       "Gemini digest prefix 未使用官方 UA normalize",
				titleUnexpected:    "Gemini digest prefix 保留了官方已移除的 UA normalize 逻辑",
				descriptionMissing: "官方 Gemini 链路命中 NormalizeSessionUserAgent，本地未命中。",
				descriptionExtra:   "本地命中 NormalizeSessionUserAgent，但官方基线未命中。",
				recommendedAction:  "replace-with-official",
				localTokens:        []string{"NormalizeSessionUserAgent"},
				officialTokens:     []string{"NormalizeSessionUserAgent"},
			}))
		case "test-file-presence":
			appendFinding(&findings, e.compareTests(ruleID, feature, decisions, input))
		}
	}

	return findings, nil
}

type ruleInput struct {
	feature          model.FeatureChain
	localContents    map[string]string
	officialContents map[string]string
}

type ruleDefinition struct {
	severity           string
	category           string
	titleMissing       string
	titleUnexpected    string
	descriptionMissing string
	descriptionExtra   string
	recommendedAction  string
	localTokens        []string
	officialTokens     []string
	localFallbackAny   []string
	useRouteNodes      bool
}

func (e Engine) buildInput(feature model.FeatureChain) (ruleInput, error) {
	localContents, err := readNodeFiles(e.localRepoRoot, feature.LocalNodes)
	if err != nil {
		return ruleInput{}, err
	}
	officialContents, err := readNodeFiles(e.officialRepoRoot, feature.OfficialNodes)
	if err != nil {
		return ruleInput{}, err
	}

	return ruleInput{
		feature:          feature,
		localContents:    localContents,
		officialContents: officialContents,
	}, nil
}

func readNodeFiles(repoRoot string, nodes []model.ChainNode) (map[string]string, error) {
	contents := make(map[string]string)
	for _, filePath := range uniqueNodeFiles(nodes) {
		absolutePath := filePath
		if repoRoot != "" && !filepath.IsAbs(filePath) {
			absolutePath = filepath.Join(repoRoot, filepath.FromSlash(filePath))
		}

		content, err := os.ReadFile(absolutePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %q: %w", absolutePath, err)
		}
		contents[filePath] = string(content)
	}
	return contents, nil
}

func uniqueNodeFiles(nodes []model.ChainNode) []string {
	files := make([]string, 0)
	seen := make(map[string]struct{})
	for _, node := range nodes {
		if strings.TrimSpace(node.FilePath) == "" {
			continue
		}
		normalized := filepath.ToSlash(node.FilePath)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		files = append(files, normalized)
	}
	return files
}

func comparePresence(ruleID string, feature model.FeatureChain, decisions []model.DecisionHint, input ruleInput, definition ruleDefinition) *model.SemanticDiff {
	localPresent := fileSetContainsAll(input.localContents, definition.localTokens)
	officialPresent := fileSetContainsAll(input.officialContents, definition.officialTokens)

	if definition.useRouteNodes {
		localPresent = localPresent || hasExtractedRoute(feature.LocalNodes, definition.localTokens...)
		officialPresent = officialPresent || hasExtractedRoute(feature.OfficialNodes, definition.officialTokens...)
	}

	if localPresent == officialPresent {
		return nil
	}

	description := definition.descriptionMissing
	title := definition.titleMissing
	if localPresent && !officialPresent {
		description = definition.descriptionExtra
		title = definition.titleUnexpected
	}

	if !localPresent && len(definition.localFallbackAny) > 0 && fileSetContainsAny(input.localContents, definition.localFallbackAny) {
		description += " 本地命中了 legacy 头写入方式。"
	}

	return &model.SemanticDiff{
		FeatureID:         feature.ID,
		Severity:          definition.severity,
		Category:          definition.category,
		Title:             title,
		Description:       description,
		Evidence:          buildEvidence(feature, input, definition.localTokens, definition.officialTokens),
		DecisionTag:       chooseDecision(feature, decisions, ruleID, feature.DefaultDecision),
		RecommendedAction: definition.recommendedAction,
	}
}

func (e Engine) compareTests(ruleID string, feature model.FeatureChain, decisions []model.DecisionHint, input ruleInput) *model.SemanticDiff {
	localTests := extractedTests(feature.LocalNodes)
	officialTests := extractedTests(feature.OfficialNodes)
	missing := missingItems(officialTests, localTests)
	if len(missing) == 0 {
		return nil
	}

	description := fmt.Sprintf("官方命中 %d 个测试节点，本地缺失 %d 个：%s", len(officialTests), len(missing), strings.Join(missing, ", "))
	return &model.SemanticDiff{
		FeatureID:         feature.ID,
		Severity:          "high",
		Category:          "test-coverage",
		Title:             "官方关键测试在本地缺失",
		Description:       description,
		Evidence:          buildEvidence(feature, input, nil, nil),
		DecisionTag:       chooseDecision(feature, decisions, ruleID, "test-missing"),
		RecommendedAction: "add-missing-test",
	}
}

func buildEvidence(feature model.FeatureChain, input ruleInput, localTokens, officialTokens []string) []model.EvidenceRef {
	evidence := make([]model.EvidenceRef, 0, 4)

	if localPath := firstMatchingFile(input.localContents, localTokens); localPath != "" {
		evidence = append(evidence, model.EvidenceRef{
			RepoSide: "local",
			FilePath: localPath,
		})
	} else if len(feature.LocalNodes) > 0 {
		evidence = append(evidence, model.EvidenceRef{
			RepoSide: "local",
			FilePath: firstNodeFile(feature.LocalNodes),
		})
	}

	if officialPath := firstMatchingFile(input.officialContents, officialTokens); officialPath != "" {
		evidence = append(evidence, model.EvidenceRef{
			RepoSide: "official",
			FilePath: officialPath,
		})
	} else if len(feature.OfficialNodes) > 0 {
		evidence = append(evidence, model.EvidenceRef{
			RepoSide: "official",
			FilePath: firstNodeFile(feature.OfficialNodes),
		})
	}

	return evidence
}

func firstMatchingFile(contents map[string]string, tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	for filePath, content := range contents {
		if containsAllTokens(content, tokens) {
			return filePath
		}
	}
	return ""
}

func firstNodeFile(nodes []model.ChainNode) string {
	for _, node := range nodes {
		if strings.TrimSpace(node.FilePath) != "" {
			return node.FilePath
		}
	}
	return ""
}

func fileSetContainsAll(contents map[string]string, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	for _, content := range contents {
		if containsAllTokens(content, tokens) {
			return true
		}
	}
	return false
}

func fileSetContainsAny(contents map[string]string, tokens []string) bool {
	for _, content := range contents {
		if containsAnyToken(content, tokens) {
			return true
		}
	}
	return false
}

func containsAllTokens(content string, tokens []string) bool {
	for _, token := range tokens {
		if !strings.Contains(content, token) {
			return false
		}
	}
	return true
}

func containsAnyToken(content string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(content, token) {
			return true
		}
	}
	return false
}

func hasExtractedRoute(nodes []model.ChainNode, tokens ...string) bool {
	for _, node := range nodes {
		if node.Kind != model.NodeKindRoute {
			continue
		}
		if !metadataBool(node.Metadata, "extracted") {
			continue
		}
		matchedLiteral, _ := node.Metadata["matchedLiteral"].(string)
		if matchedLiteral == "" {
			continue
		}
		if containsAllTokens(matchedLiteral, tokens) {
			return true
		}
	}
	return false
}

func extractedTests(nodes []model.ChainNode) []string {
	tests := make([]string, 0)
	for _, node := range nodes {
		if node.Kind != model.NodeKindTest || !metadataBool(node.Metadata, "extracted") {
			continue
		}
		key := node.FilePath
		if node.SymbolName != "" {
			key += "#" + node.SymbolName
		}
		tests = append(tests, key)
	}
	slices.Sort(tests)
	return tests
}

func missingItems(expected, actual []string) []string {
	actualSet := make(map[string]struct{}, len(actual))
	for _, item := range actual {
		actualSet[item] = struct{}{}
	}

	missing := make([]string, 0)
	for _, item := range expected {
		if _, ok := actualSet[item]; !ok {
			missing = append(missing, item)
		}
	}
	return missing
}

func chooseDecision(feature model.FeatureChain, decisions []model.DecisionHint, scope, fallback string) string {
	for _, hint := range decisions {
		if hint.FeatureID != feature.ID {
			continue
		}
		if hint.Scope == scope || hint.Scope == "*" {
			return hint.Decision
		}
	}
	if fallback != "" {
		return fallback
	}
	if feature.DefaultDecision != "" {
		return feature.DefaultDecision
	}
	return "manual-merge"
}

func appendFinding(findings *[]model.SemanticDiff, finding *model.SemanticDiff) {
	if finding != nil {
		*findings = append(*findings, *finding)
	}
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

func WalkGoFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".forktool", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(entry.Name()) != ".go" {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relativePath))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
