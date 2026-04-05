package configx

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
	return "configx"
}

func (Adapter) SupportsLanguage(lang string) bool {
	return strings.EqualFold(lang, "yaml") || strings.EqualFold(lang, "json")
}

func (Adapter) Discover(_ context.Context, req discovery.DiscoverRequest) ([]model.ChainNode, error) {
	candidateFiles, err := textscan.WalkFiles(req.RepoRoot, isConfigCandidate)
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
		result, matched, err := textscan.Scan(req.RepoRoot, relativePath, hints, 4)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}

		candidates = append(candidates, candidate{
			score: result.Score,
			node: model.ChainNode{
				Kind:       model.NodeKindConfig,
				Language:   languageForFile(relativePath),
				FilePath:   filepath.ToSlash(relativePath),
				SymbolName: configSymbolName(relativePath),
				Range: model.SourceRange{
					StartLine: result.StartLine,
					EndLine:   result.EndLine,
				},
				Metadata: map[string]any{
					"repoSide":      req.RepoSide,
					"adapter":       "configx",
					"exists":        true,
					"extracted":     true,
					"source":        "text-config-scan",
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

	nodes := make([]model.ChainNode, 0, minInt(len(candidates), 6))
	for _, candidate := range candidates[:minInt(len(candidates), 6)] {
		nodes = append(nodes, candidate.node)
	}
	return nodes, nil
}

func isConfigCandidate(relativePath string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(relativePath))
	switch {
	case strings.HasSuffix(lowerPath, ".yaml"), strings.HasSuffix(lowerPath, ".yml"):
		return strings.HasPrefix(lowerPath, "deploy/") ||
			strings.Contains(lowerPath, "/deploy/") ||
			strings.Contains(lowerPath, "/config") ||
			strings.Contains(lowerPath, "config.") ||
			strings.Contains(lowerPath, "docker-compose")
	case strings.HasSuffix(lowerPath, ".json"):
		return strings.Contains(lowerPath, "/resources/") ||
			strings.Contains(lowerPath, "/config") ||
			strings.Contains(lowerPath, "/settings")
	default:
		return false
	}
}

func languageForFile(relativePath string) string {
	switch strings.ToLower(filepath.Ext(relativePath)) {
	case ".json":
		return "json"
	default:
		return "yaml"
	}
}

func configSymbolName(relativePath string) string {
	base := strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath))
	return strings.TrimSpace(base)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
