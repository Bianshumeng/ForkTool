package markdownx

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"forktool/internal/adapters/textscan"
	"forktool/internal/discovery"
	"forktool/pkg/model"
)

type Adapter struct{}

func New() discovery.Adapter {
	return Adapter{}
}

func (Adapter) Name() string {
	return "markdownx"
}

func (Adapter) SupportsLanguage(lang string) bool {
	return strings.EqualFold(lang, "markdown")
}

func (Adapter) Discover(_ context.Context, req discovery.DiscoverRequest) ([]model.ChainNode, error) {
	candidateFiles, err := textscan.WalkFiles(req.RepoRoot, isMarkdownCandidate)
	if err != nil {
		return nil, err
	}

	hints := discovery.KeywordHints(req.Feature)
	type candidate struct {
		node  model.ChainNode
		score int
	}
	candidates := make([]candidate, 0)
	for _, relativePath := range candidateFiles {
		result, matched, err := textscan.Scan(req.RepoRoot, relativePath, hints, 3)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}

		candidates = append(candidates, candidate{
			score: result.Score,
			node: model.ChainNode{
				Kind:       model.NodeKindDoc,
				Language:   "markdown",
				FilePath:   filepath.ToSlash(relativePath),
				SymbolName: strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath)),
				Range: model.SourceRange{
					StartLine: result.StartLine,
					EndLine:   result.EndLine,
				},
				Metadata: map[string]any{
					"repoSide":      req.RepoSide,
					"adapter":       "markdownx",
					"exists":        true,
					"extracted":     true,
					"source":        "text-markdown-scan",
					"matchedTokens": result.MatchedTokens,
					"score":         result.Score,
					"snippet":       result.Snippet,
					"decisionTags":  extractDecisionTags(result.Snippet),
				},
			},
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].node.FilePath < candidates[j].node.FilePath
		}
		return candidates[i].score > candidates[j].score
	})

	nodes := make([]model.ChainNode, 0, minInt(len(candidates), 8))
	for _, candidate := range candidates[:minInt(len(candidates), 8)] {
		nodes = append(nodes, candidate.node)
	}
	return nodes, nil
}

func isMarkdownCandidate(relativePath string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(relativePath))
	if !strings.HasSuffix(lowerPath, ".md") {
		return false
	}

	return strings.HasPrefix(lowerPath, "docs/") ||
		strings.Contains(lowerPath, "/docs/") ||
		strings.HasPrefix(lowerPath, "plan/") ||
		strings.Contains(lowerPath, "/plan/") ||
		strings.HasPrefix(lowerPath, "说明文档/") ||
		strings.Contains(lowerPath, "/说明文档/") ||
		strings.HasPrefix(lowerPath, "frontend/content/") ||
		strings.Contains(lowerPath, "/frontend/content/") ||
		strings.HasSuffix(lowerPath, "readme.md")
}

func extractDecisionTags(snippet string) []string {
	candidates := []string{
		"keep-local",
		"manual-merge",
		"prefer-official",
		"officialized",
		"official-required",
	}

	found := make([]string, 0, len(candidates))
	lowerSnippet := strings.ToLower(snippet)
	for _, candidate := range candidates {
		if strings.Contains(lowerSnippet, candidate) {
			found = append(found, candidate)
		}
	}
	return found
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
