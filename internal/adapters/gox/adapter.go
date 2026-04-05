package gox

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"forktool/internal/discovery"
	"forktool/pkg/model"
)

type Adapter struct{}

type parsedGoFile struct {
	AbsolutePath string
	RelativePath string
	FileSet      *token.FileSet
	File         *ast.File
}

type parserCache struct {
	repoRoot string
	files    map[string]*parsedGoFile
}

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
	cache := newParserCache(req.RepoRoot)

	routeNodes, err := discoverRoutes(cache, req.Feature.Chain.Routes, req.RepoSide)
	if err != nil {
		return nil, err
	}

	symbolNodes, err := discoverSymbols(cache, req.Feature.Chain.Symbols, req.RepoSide)
	if err != nil {
		return nil, err
	}

	testNodes, err := discoverTests(cache, req.Feature.AllTests(), req.RepoSide)
	if err != nil {
		return nil, err
	}

	nodes := make([]model.ChainNode, 0, len(routeNodes)+len(symbolNodes)+len(testNodes))
	nodes = append(nodes, routeNodes...)
	nodes = append(nodes, symbolNodes...)
	nodes = append(nodes, testNodes...)
	return nodes, nil
}

func newParserCache(repoRoot string) *parserCache {
	return &parserCache{
		repoRoot: repoRoot,
		files:    make(map[string]*parsedGoFile),
	}
}

func (c *parserCache) parse(relativeOrAbsolutePath string) (*parsedGoFile, error) {
	absolutePath := resolvePath(c.repoRoot, relativeOrAbsolutePath)
	if cached, ok := c.files[absolutePath]; ok {
		return cached, nil
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, absolutePath, nil, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse go file %q: %w", absolutePath, err)
	}

	relativePath := filepath.ToSlash(relativeOrAbsolutePath)
	if filepath.IsAbs(relativeOrAbsolutePath) {
		if rel, relErr := filepath.Rel(c.repoRoot, absolutePath); relErr == nil {
			relativePath = filepath.ToSlash(rel)
		} else {
			relativePath = filepath.ToSlash(absolutePath)
		}
	}

	cached := &parsedGoFile{
		AbsolutePath: absolutePath,
		RelativePath: relativePath,
		FileSet:      fileSet,
		File:         parsed,
	}
	c.files[absolutePath] = cached
	return cached, nil
}

func discoverRoutes(cache *parserCache, routes []model.ManifestRoute, repoSide string) ([]model.ChainNode, error) {
	goFiles, err := collectGoFiles(cache.repoRoot)
	if err != nil {
		return nil, err
	}

	nodes := make([]model.ChainNode, 0, len(routes))
	for _, route := range routes {
		matched := false
		for _, goFile := range goFiles {
			parsed, err := cache.parse(goFile)
			if err != nil {
				return nil, err
			}

			matches := collectRouteMatches(parsed, route.PathPattern, repoSide)
			if len(matches) > 0 {
				matched = true
				nodes = append(nodes, matches...)
			}
		}

		if !matched {
			nodes = append(nodes, model.ChainNode{
				Kind:     model.NodeKindRoute,
				Language: "go",
				Metadata: map[string]any{
					"pathPattern": route.PathPattern,
					"repoSide":    repoSide,
					"adapter":     "gox",
					"exists":      false,
					"extracted":   false,
					"source":      "manifest-fallback",
				},
			})
		}
	}

	return nodes, nil
}

func collectRouteMatches(parsed *parsedGoFile, pathPattern, repoSide string) []model.ChainNode {
	nodes := make([]model.ChainNode, 0)

	for _, decl := range parsed.File.Decls {
		switch typedDecl := decl.(type) {
		case *ast.FuncDecl:
			if typedDecl.Body == nil {
				continue
			}
			if !isRouteContext(parsed.RelativePath, typedDecl.Name.Name) {
				continue
			}

			ast.Inspect(typedDecl.Body, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}

				unquoted, err := strconv.Unquote(literal.Value)
				if err != nil || !routePatternMatches(pathPattern, unquoted) {
					return true
				}

				nodes = append(nodes, model.ChainNode{
					Kind:       model.NodeKindRoute,
					Language:   "go",
					FilePath:   parsed.RelativePath,
					SymbolName: typedDecl.Name.Name,
					Range:      sourceRange(parsed.FileSet, literal.Pos(), literal.End()),
					Metadata: map[string]any{
						"pathPattern":    pathPattern,
						"matchedLiteral": unquoted,
						"repoSide":       repoSide,
						"adapter":        "gox",
						"exists":         true,
						"extracted":      true,
						"source":         "go-ast",
					},
				})
				return true
			})
		case *ast.GenDecl:
			if !isRouteFile(parsed.RelativePath) {
				continue
			}
			for _, spec := range typedDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}

				for _, value := range valueSpec.Values {
					literal, ok := value.(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						continue
					}

					unquoted, err := strconv.Unquote(literal.Value)
					if err != nil || !routePatternMatches(pathPattern, unquoted) {
						continue
					}

					nodes = append(nodes, model.ChainNode{
						Kind:     model.NodeKindRoute,
						Language: "go",
						FilePath: parsed.RelativePath,
						Range:    sourceRange(parsed.FileSet, literal.Pos(), literal.End()),
						Metadata: map[string]any{
							"pathPattern":    pathPattern,
							"matchedLiteral": unquoted,
							"repoSide":       repoSide,
							"adapter":        "gox",
							"exists":         true,
							"extracted":      true,
							"source":         "go-ast",
						},
					})
				}
			}
		}
	}

	return nodes
}

func discoverSymbols(cache *parserCache, symbols []model.ManifestSymbol, repoSide string) ([]model.ChainNode, error) {
	nodes := make([]model.ChainNode, 0)

	for _, symbol := range symbols {
		symbolPath := resolvePath(cache.repoRoot, symbol.File)
		exists := fileExists(symbolPath)
		if !exists {
			for _, functionName := range symbol.Functions {
				nodes = append(nodes, model.ChainNode{
					Kind:       kindForSymbol(symbol.File, functionName),
					Language:   "go",
					FilePath:   filepath.ToSlash(symbol.File),
					SymbolName: functionName,
					Metadata: map[string]any{
						"repoSide":  repoSide,
						"adapter":   "gox",
						"exists":    false,
						"extracted": false,
						"source":    "manifest-fallback",
					},
				})
			}
			continue
		}

		parsed, err := cache.parse(symbol.File)
		if err != nil {
			return nil, err
		}

		functions := map[string]*ast.FuncDecl{}
		for _, decl := range parsed.File.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Name == nil {
				continue
			}
			functions[funcDecl.Name.Name] = funcDecl
		}

		for _, functionName := range symbol.Functions {
			funcDecl, ok := functions[functionName]
			if !ok {
				nodes = append(nodes, model.ChainNode{
					Kind:       kindForSymbol(symbol.File, functionName),
					Language:   "go",
					FilePath:   filepath.ToSlash(symbol.File),
					SymbolName: functionName,
					Metadata: map[string]any{
						"repoSide":  repoSide,
						"adapter":   "gox",
						"exists":    true,
						"extracted": false,
						"source":    "manifest-fallback",
					},
				})
				continue
			}

			nodes = append(nodes, model.ChainNode{
				Kind:       kindForSymbol(symbol.File, functionName),
				Language:   "go",
				FilePath:   parsed.RelativePath,
				SymbolName: functionName,
				Range:      sourceRange(parsed.FileSet, funcDecl.Pos(), funcDecl.End()),
				Metadata: map[string]any{
					"repoSide":  repoSide,
					"adapter":   "gox",
					"exists":    true,
					"extracted": true,
					"source":    "go-ast",
				},
			})
		}
	}

	return nodes, nil
}

func discoverTests(cache *parserCache, testFiles []string, repoSide string) ([]model.ChainNode, error) {
	nodes := make([]model.ChainNode, 0)

	for _, testFile := range testFiles {
		absolutePath := resolvePath(cache.repoRoot, testFile)
		exists := fileExists(absolutePath)
		if !exists {
			nodes = append(nodes, model.ChainNode{
				Kind:     model.NodeKindTest,
				Language: "go",
				FilePath: filepath.ToSlash(testFile),
				Metadata: map[string]any{
					"repoSide":  repoSide,
					"adapter":   "gox",
					"exists":    false,
					"extracted": false,
					"source":    "manifest-fallback",
				},
			})
			continue
		}

		parsed, err := cache.parse(testFile)
		if err != nil {
			return nil, err
		}

		testFunctionCount := 0
		for _, decl := range parsed.File.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Name == nil || !strings.HasPrefix(funcDecl.Name.Name, "Test") {
				continue
			}

			testFunctionCount++
			nodes = append(nodes, model.ChainNode{
				Kind:       model.NodeKindTest,
				Language:   "go",
				FilePath:   parsed.RelativePath,
				SymbolName: funcDecl.Name.Name,
				Range:      sourceRange(parsed.FileSet, funcDecl.Pos(), funcDecl.End()),
				Metadata: map[string]any{
					"repoSide":  repoSide,
					"adapter":   "gox",
					"exists":    true,
					"extracted": true,
					"source":    "go-ast",
				},
			})
		}

		if testFunctionCount == 0 {
			nodes = append(nodes, model.ChainNode{
				Kind:     model.NodeKindTest,
				Language: "go",
				FilePath: parsed.RelativePath,
				Metadata: map[string]any{
					"repoSide":          repoSide,
					"adapter":           "gox",
					"exists":            true,
					"extracted":         false,
					"source":            "manifest-fallback",
					"testFunctionCount": 0,
				},
			})
		}
	}

	return nodes, nil
}

func collectGoFiles(root string) ([]string, error) {
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
		return nil, fmt.Errorf("walk go files: %w", err)
	}

	return files, nil
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
	case strings.Contains(lowerFile, "compat"), strings.Contains(lowerFile, "helper"):
		return model.NodeKindHelper
	case strings.Contains(lowerFunction, "helper"), strings.Contains(lowerFunction, "transform"):
		return model.NodeKindHelper
	default:
		return model.NodeKindService
	}
}

func routePatternMatches(pathPattern, literal string) bool {
	pathPattern = strings.TrimSpace(pathPattern)
	literal = strings.TrimSpace(literal)
	if pathPattern == "" || literal == "" {
		return false
	}

	if strings.Contains(pathPattern, "*") {
		prefix := strings.TrimSuffix(pathPattern, "*")
		return strings.Contains(literal, prefix)
	}

	return literal == pathPattern || strings.Contains(literal, pathPattern)
}

func isRouteContext(filePath, functionName string) bool {
	return isRouteFile(filePath) || isRouteFunction(functionName)
}

func isRouteFile(filePath string) bool {
	lowerFile := strings.ToLower(filePath)
	return strings.Contains(lowerFile, "route") || strings.Contains(lowerFile, "router")
}

func isRouteFunction(functionName string) bool {
	lowerFunction := strings.ToLower(functionName)
	return strings.Contains(lowerFunction, "route") || strings.Contains(lowerFunction, "register")
}

func sourceRange(fileSet *token.FileSet, start, end token.Pos) model.SourceRange {
	startPos := fileSet.Position(start)
	endPos := fileSet.Position(end)
	return model.SourceRange{
		StartLine:   startPos.Line,
		StartColumn: startPos.Column,
		EndLine:     endPos.Line,
		EndColumn:   endPos.Column,
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
