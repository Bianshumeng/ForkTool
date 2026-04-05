package tsvue

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
	return "tsvue"
}

func (Adapter) SupportsLanguage(lang string) bool {
	return strings.EqualFold(lang, "ts") ||
		strings.EqualFold(lang, "typescript") ||
		strings.EqualFold(lang, "vue")
}

func (Adapter) Discover(_ context.Context, req discovery.DiscoverRequest) ([]model.ChainNode, error) {
	candidateFiles, err := textscan.WalkFiles(req.RepoRoot, isFrontendCandidate)
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
				Kind:       kindForFrontendFile(relativePath),
				Language:   languageForFrontendFile(relativePath),
				FilePath:   filepath.ToSlash(relativePath),
				SymbolName: strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath)),
				Range: model.SourceRange{
					StartLine: result.StartLine,
					EndLine:   result.EndLine,
				},
				Metadata: map[string]any{
					"repoSide":      req.RepoSide,
					"adapter":       "tsvue",
					"exists":        true,
					"extracted":     true,
					"source":        "text-frontend-scan",
					"matchedTokens": result.MatchedTokens,
					"score":         result.Score,
					"snippet":       result.Snippet,
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

	nodes := make([]model.ChainNode, 0, minInt(len(candidates), 12))
	for _, candidate := range candidates[:minInt(len(candidates), 12)] {
		nodes = append(nodes, candidate.node)
	}
	return nodes, nil
}

func isFrontendCandidate(relativePath string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(relativePath))
	switch {
	case strings.HasSuffix(lowerPath, ".vue"):
		return strings.HasPrefix(lowerPath, "frontend/src/") || strings.Contains(lowerPath, "/frontend/src/")
	case strings.HasSuffix(lowerPath, ".ts"), strings.HasSuffix(lowerPath, ".tsx"):
		return strings.HasPrefix(lowerPath, "frontend/src/router/") ||
			strings.Contains(lowerPath, "/frontend/src/router/") ||
			strings.HasPrefix(lowerPath, "frontend/src/api/") ||
			strings.Contains(lowerPath, "/frontend/src/api/") ||
			strings.HasPrefix(lowerPath, "frontend/src/stores/") ||
			strings.Contains(lowerPath, "/frontend/src/stores/") ||
			strings.HasPrefix(lowerPath, "frontend/src/composables/") ||
			strings.Contains(lowerPath, "/frontend/src/composables/") ||
			strings.HasPrefix(lowerPath, "frontend/src/types/") ||
			strings.Contains(lowerPath, "/frontend/src/types/") ||
			strings.HasPrefix(lowerPath, "frontend/src/components/layout/sidebar/") ||
			strings.Contains(lowerPath, "/frontend/src/components/layout/sidebar/")
	default:
		return false
	}
}

func kindForFrontendFile(relativePath string) model.NodeKind {
	lowerPath := strings.ToLower(filepath.ToSlash(relativePath))
	switch {
	case strings.Contains(lowerPath, "/router/"), strings.Contains(lowerPath, "/layout/sidebar/"):
		return model.NodeKindNav
	case strings.Contains(lowerPath, "/views/"):
		return model.NodeKindPage
	case strings.Contains(lowerPath, "/api/"):
		return model.NodeKindAPI
	case strings.Contains(lowerPath, "/types/"):
		return model.NodeKindDTO
	case strings.Contains(lowerPath, "/stores/"), strings.Contains(lowerPath, "/composables/"):
		return model.NodeKindStore
	default:
		return model.NodeKindPage
	}
}

func languageForFrontendFile(relativePath string) string {
	if strings.EqualFold(filepath.Ext(relativePath), ".vue") {
		return "vue"
	}
	return "ts"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
