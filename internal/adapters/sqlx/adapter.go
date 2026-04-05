package sqlx

import (
	"context"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"forktool/internal/adapters/textscan"
	"forktool/internal/discovery"
	"forktool/pkg/model"
)

type Adapter struct{}

var sqlObjectPattern = regexp.MustCompile(`(?i)\b(?:create|alter)\s+(?:table|index)\s+("?[\w.]+"?)`)

func New() discovery.Adapter {
	return Adapter{}
}

func (Adapter) Name() string {
	return "sqlx"
}

func (Adapter) SupportsLanguage(lang string) bool {
	return strings.EqualFold(lang, "sql")
}

func (Adapter) Discover(_ context.Context, req discovery.DiscoverRequest) ([]model.ChainNode, error) {
	candidateFiles, err := textscan.WalkFiles(req.RepoRoot, isSQLCandidate)
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
		result, matched, err := textscan.Scan(req.RepoRoot, relativePath, hints, 2)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}

		objects, objectErr := extractSQLObjects(req.RepoRoot, relativePath)
		if objectErr != nil {
			return nil, objectErr
		}

		candidates = append(candidates, candidate{
			score: result.Score,
			node: model.ChainNode{
				Kind:       model.NodeKindMigration,
				Language:   "sql",
				FilePath:   filepath.ToSlash(relativePath),
				SymbolName: strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath)),
				Range: model.SourceRange{
					StartLine: result.StartLine,
					EndLine:   result.EndLine,
				},
				Metadata: map[string]any{
					"repoSide":      req.RepoSide,
					"adapter":       "sqlx",
					"exists":        true,
					"extracted":     true,
					"source":        "text-sql-scan",
					"matchedTokens": result.MatchedTokens,
					"score":         result.Score,
					"snippet":       result.Snippet,
					"objects":       objects,
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

func isSQLCandidate(relativePath string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(relativePath))
	return strings.HasSuffix(lowerPath, ".sql") &&
		(strings.Contains(lowerPath, "/migrations/") || strings.Contains(lowerPath, "/docs/"))
}

func extractSQLObjects(repoRoot, relativePath string) ([]string, error) {
	result, matched, err := textscan.Scan(repoRoot, relativePath, []string{"create table", "alter table", "create index"}, 1)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, nil
	}

	objects := make([]string, 0)
	for _, match := range sqlObjectPattern.FindAllStringSubmatch(result.Snippet, -1) {
		if len(match) < 2 {
			continue
		}
		objects = append(objects, strings.Trim(match[1], `"`))
	}
	return uniqueStrings(objects), nil
}

func uniqueStrings(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
