package gox

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"forktool/internal/discovery"
	"forktool/pkg/model"
)

type Adapter struct{}

func New() discovery.Adapter {
	return Adapter{}
}

func (Adapter) Name() string {
	return "gox"
}

func (Adapter) SupportsLanguage(lang string) bool {
	return strings.EqualFold(lang, "go")
}

func (Adapter) Discover(_ context.Context, req discovery.DiscoverRequest) ([]model.ChainNode, error) {
	nodes := make([]model.ChainNode, 0, len(req.Feature.Chain.Routes)+len(req.Feature.Chain.Symbols)+len(req.Feature.AllTests()))

	for _, route := range req.Feature.Chain.Routes {
		nodes = append(nodes, model.ChainNode{
			Kind:     model.NodeKindRoute,
			Language: "go",
			Metadata: map[string]any{
				"pathPattern": route.PathPattern,
				"repoSide":    req.RepoSide,
				"source":      "manifest",
			},
		})
	}

	for _, symbol := range req.Feature.Chain.Symbols {
		symbolPath := resolvePath(req.RepoRoot, symbol.File)
		exists := fileExists(symbolPath)
		for _, functionName := range symbol.Functions {
			nodes = append(nodes, model.ChainNode{
				Kind:       kindForSymbol(symbol.File, functionName),
				Language:   "go",
				FilePath:   filepath.ToSlash(symbol.File),
				SymbolName: functionName,
				Metadata: map[string]any{
					"exists":   exists,
					"repoSide": req.RepoSide,
					"adapter":  "gox",
				},
			})
		}
	}

	for _, testFile := range req.Feature.AllTests() {
		nodes = append(nodes, model.ChainNode{
			Kind:     model.NodeKindTest,
			Language: "go",
			FilePath: filepath.ToSlash(testFile),
			Metadata: map[string]any{
				"exists":   fileExists(resolvePath(req.RepoRoot, testFile)),
				"repoSide": req.RepoSide,
				"adapter":  "gox",
			},
		})
	}

	return nodes, nil
}

func kindForSymbol(filePath, functionName string) model.NodeKind {
	lowerFile := strings.ToLower(filePath)
	lowerFunction := strings.ToLower(functionName)

	switch {
	case strings.Contains(lowerFile, "handler"):
		return model.NodeKindHandler
	case strings.Contains(lowerFile, "service"):
		return model.NodeKindService
	case strings.Contains(lowerFile, "route"):
		return model.NodeKindRoute
	case strings.Contains(lowerFunction, "helper"), strings.Contains(lowerFunction, "transform"):
		return model.NodeKindHelper
	default:
		return model.NodeKindService
	}
}

func resolvePath(root, relative string) string {
	if relative == "" {
		return ""
	}
	relative = filepath.FromSlash(relative)
	if filepath.IsAbs(relative) || root == "" {
		return relative
	}
	return filepath.Join(root, relative)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
