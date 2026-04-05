package textscan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Result struct {
	MatchedTokens []string
	Score         int
	StartLine     int
	EndLine       int
	Snippet       string
}

func WalkFiles(root string, include func(relativePath string) bool) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(relativePath)
		if include != nil && !include(normalized) {
			return nil
		}
		files = append(files, normalized)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk files: %w", err)
	}

	sort.Strings(files)
	return files, nil
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case ".git", ".forktool", ".cache", "node_modules", "vendor", ".beads", ".go-work", ".gocache", ".gopath", "%gomodcache%", "%gopath%", "%systemdrive%", "${appdata}", ".codex", ".codex-cache", ".shared":
		return true
	}

	return strings.HasPrefix(strings.ToLower(name), ".tmp")
}

func Scan(root, relativePath string, hints []string, minScore int) (Result, bool, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
	if err != nil {
		return Result{}, false, fmt.Errorf("read %q: %w", relativePath, err)
	}

	result := scanContent(relativePath, string(content), hints)
	if result.Score < minScore {
		return Result{}, false, nil
	}

	return result, true, nil
}

func scanContent(relativePath, content string, hints []string) Result {
	lines := strings.Split(content, "\n")
	lowerContent := strings.ToLower(content)
	lowerPath := strings.ToLower(relativePath)

	matchedTokens := make([]string, 0, len(hints))
	firstLine := 0
	lastLine := 0
	score := 0

	for _, hint := range hints {
		hint = strings.ToLower(strings.TrimSpace(hint))
		if len(hint) < 2 {
			continue
		}

		pathHit := strings.Contains(lowerPath, hint)
		contentHit := strings.Contains(lowerContent, hint)
		if !pathHit && !contentHit {
			continue
		}

		matchedTokens = append(matchedTokens, hint)
		switch {
		case pathHit && contentHit:
			score += 4
		case contentHit:
			score += 3
		default:
			score++
		}

		if contentHit {
			for index, line := range lines {
				if !strings.Contains(strings.ToLower(line), hint) {
					continue
				}
				lineNumber := index + 1
				if firstLine == 0 || lineNumber < firstLine {
					firstLine = lineNumber
				}
				if lineNumber > lastLine {
					lastLine = lineNumber
				}
				break
			}
		}
	}

	if firstLine == 0 {
		firstLine = 1
	}
	if lastLine == 0 {
		lastLine = minInt(len(lines), firstLine+2)
	}

	return Result{
		MatchedTokens: uniqueStrings(matchedTokens),
		Score:         score,
		StartLine:     firstLine,
		EndLine:       lastLine,
		Snippet:       snippet(lines, firstLine, lastLine),
	}
}

func snippet(lines []string, startLine, endLine int) string {
	if len(lines) == 0 {
		return ""
	}
	if startLine <= 0 {
		startLine = 1
	}
	if endLine < startLine {
		endLine = startLine
	}

	startIndex := maxInt(0, startLine-1)
	endIndex := minInt(len(lines), endLine)
	segment := lines[startIndex:endIndex]
	return strings.TrimSpace(strings.Join(segment, "\n"))
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
