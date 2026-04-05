package discovery

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"forktool/pkg/model"
)

var nonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)

var featureSpecificHints = map[string][]string{
	"claude-messages-mainchain": {"claude", "anthropic", "messages", "gateway", "metadata", "session"},
	"claude-count-tokens":       {"claude", "anthropic", "messages", "count_tokens", "beta", "gateway"},
	"openai-responses-http":     {"openai", "responses", "originator", "passthrough", "store", "stream"},
	"openai-responses-compact":  {"openai", "responses", "compact", "passthrough", "store", "stream"},
	"openai-responses-ws":       {"openai", "responses", "ws", "websocket", "replay", "previous_response_id"},
	"gemini-native-v1beta":      {"gemini", "google", "v1beta", "models", "modelaction"},
	"gemini-messages-compat":    {"gemini", "compat", "messages", "upstream_model", "usage"},
}

var ruleSpecificHints = map[string][]string{
	"claude-metadata-userid-format":         {"metadata", "user_id", "oauth"},
	"claude-count-tokens-beta-suffix":       {"count_tokens", "beta"},
	"claude-session-hash-normalization":     {"session", "normalize", "useragent"},
	"http-header-wire-casing":               {"header", "wire", "casing"},
	"response-header-filter":                {"response_headers", "responseheaderfilter", "writefilteredheaders"},
	"openai-compact-path-suffix":            {"compact", "responses"},
	"openai-originator-compatibility":       {"originator", "openai-beta"},
	"openai-session-isolation":              {"session", "conversation", "isolate"},
	"openai-passthrough-body-normalization": {"passthrough", "body", "store", "stream"},
	"openai-ws-previous-response-id":        {"websocket", "previous_response_id", "responses"},
	"openai-ws-turn-metadata-replay":        {"websocket", "turn", "metadata", "replay"},
	"observability-upstream-model":          {"requested_model", "upstream_model", "service_tier", "endpoint"},
	"gemini-failover-semantics":             {"gemini", "failover"},
	"gemini-upstream-model-preserved":       {"gemini", "upstream_model"},
	"gemini-digest-prefix-ua-normalization": {"gemini", "digest", "normalize", "useragent"},
	"test-file-presence":                    {"test"},
}

func KeywordHints(feature model.ManifestFeature) []string {
	hints := make([]string, 0, 32)
	hints = append(hints, featureSpecificHints[feature.ID]...)
	hints = append(hints, normalizeTokens(feature.ID)...)
	hints = append(hints, normalizeTokens(feature.Name)...)

	for _, ruleID := range feature.SemanticRules {
		hints = append(hints, ruleSpecificHints[ruleID]...)
		hints = append(hints, normalizeTokens(ruleID)...)
	}

	for _, route := range feature.Chain.Routes {
		hints = append(hints, routeTokens(route.PathPattern)...)
	}

	for _, symbol := range feature.Chain.Symbols {
		base := strings.TrimSuffix(filepath.Base(symbol.File), filepath.Ext(symbol.File))
		hints = append(hints, normalizeTokens(base)...)
		for _, functionName := range symbol.Functions {
			hints = append(hints, normalizeTokens(functionName)...)
		}
	}

	for _, testPath := range feature.AllTests() {
		base := strings.TrimSuffix(filepath.Base(testPath), filepath.Ext(testPath))
		hints = append(hints, normalizeTokens(base)...)
	}

	return uniqueHints(hints)
}

func HasLanguage(feature model.ManifestFeature, languages ...string) bool {
	if len(feature.Languages) == 0 || len(languages) == 0 {
		return false
	}

	for _, candidate := range feature.Languages {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		for _, language := range languages {
			if candidate == strings.ToLower(strings.TrimSpace(language)) {
				return true
			}
		}
	}

	return false
}

func routeTokens(pathPattern string) []string {
	pathPattern = strings.TrimSpace(pathPattern)
	if pathPattern == "" {
		return nil
	}

	parts := strings.FieldsFunc(pathPattern, func(r rune) bool {
		switch r {
		case '/', '*', ':', '-', '_', '.', '?', '=', '&':
			return true
		default:
			return false
		}
	})

	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		tokens = append(tokens, normalizeTokens(part)...)
	}
	return uniqueHints(tokens)
}

func normalizeTokens(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	var builder strings.Builder
	lastWasLower := false
	for _, r := range value {
		if unicode.IsUpper(r) && lastWasLower {
			builder.WriteByte(' ')
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(unicode.ToLower(r))
			lastWasLower = unicode.IsLower(r)
			continue
		}
		builder.WriteByte(' ')
		lastWasLower = false
	}

	normalized := nonAlphaNumeric.ReplaceAllString(builder.String(), " ")
	parts := strings.Fields(normalized)
	tokens := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		if len(part) < 2 {
			continue
		}
		tokens = append(tokens, part)
		if strings.Contains(part, " ") {
			continue
		}
	}
	return uniqueHints(tokens)
}

func uniqueHints(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if len(value) < 2 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
