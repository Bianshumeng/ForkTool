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
	"slices"
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

type routeRegistration struct {
	Method         string
	Path           string
	FullPath       string
	HandlerTargets []string
	PrimaryHandler string
	Range          model.SourceRange
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

	derivedHandlerNodes, err := discoverDerivedHandlers(cache, routeNodes, symbolNodes, req.RepoSide)
	if err != nil {
		return nil, err
	}

	nodes := make([]model.ChainNode, 0, len(routeNodes)+len(symbolNodes)+len(testNodes)+len(derivedHandlerNodes))
	nodes = append(nodes, routeNodes...)
	nodes = append(nodes, derivedHandlerNodes...)
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
	candidateFiles, err := collectRouteFiles(cache.repoRoot)
	if err != nil {
		return nil, err
	}

	nodes := make([]model.ChainNode, 0, len(routes))
	for _, route := range routes {
		matched := false
		for _, goFile := range candidateFiles {
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
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil || funcDecl.Name == nil {
			continue
		}

		for _, registration := range collectRouteRegistrations(parsed, funcDecl) {
			if !routePatternMatches(pathPattern, registration.FullPath) {
				continue
			}

			metadata := map[string]any{
				"pathPattern":     pathPattern,
				"matchedLiteral":  registration.Path,
				"matchedFullPath": registration.FullPath,
				"method":          registration.Method,
				"repoSide":        repoSide,
				"adapter":         "gox",
				"exists":          true,
				"extracted":       true,
				"source":          "go-ast",
			}
			if len(registration.HandlerTargets) > 0 {
				metadata["handlerTargets"] = registration.HandlerTargets
				metadata["primaryHandler"] = registration.PrimaryHandler
			}

			relations := make([]model.NodeRelation, 0)
			if registration.PrimaryHandler != "" {
				relations = append(relations, model.NodeRelation{
					Type:   "handles",
					Target: registration.PrimaryHandler,
				})
			}

			nodes = append(nodes, model.ChainNode{
				Kind:       model.NodeKindRoute,
				Language:   "go",
				FilePath:   parsed.RelativePath,
				SymbolName: funcDecl.Name.Name,
				Range:      registration.Range,
				Metadata:   metadata,
				Relations:  relations,
			})
		}
	}

	return nodes
}

func collectRouteRegistrations(parsed *parsedGoFile, funcDecl *ast.FuncDecl) []routeRegistration {
	prefixes := map[string]string{}
	for _, param := range functionParamNames(funcDecl) {
		prefixes[param] = ""
	}

	return inspectRouteStatements(parsed, funcDecl.Body.List, prefixes)
}

func inspectRouteStatements(parsed *parsedGoFile, statements []ast.Stmt, prefixes map[string]string) []routeRegistration {
	registrations := make([]routeRegistration, 0)
	currentPrefixes := clonePrefixes(prefixes)

	for _, statement := range statements {
		switch typedStmt := statement.(type) {
		case *ast.AssignStmt:
			for index, rhs := range typedStmt.Rhs {
				if prefix, ok := resolveGroupPrefix(rhs, currentPrefixes); ok && index < len(typedStmt.Lhs) {
					if ident, ok := typedStmt.Lhs[index].(*ast.Ident); ok {
						currentPrefixes[ident.Name] = prefix
					}
				}

				if call, ok := rhs.(*ast.CallExpr); ok {
					if registration, ok := buildRouteRegistration(parsed, call, currentPrefixes); ok {
						registrations = append(registrations, registration)
					}
				}
			}
		case *ast.DeclStmt:
			genDecl, ok := typedStmt.Decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, value := range valueSpec.Values {
					if prefix, ok := resolveGroupPrefix(value, currentPrefixes); ok && index < len(valueSpec.Names) {
						currentPrefixes[valueSpec.Names[index].Name] = prefix
					}
				}
			}
		case *ast.ExprStmt:
			registrations = append(registrations, extractRegistrationsFromExpr(parsed, typedStmt.X, currentPrefixes)...)
		case *ast.BlockStmt:
			registrations = append(registrations, inspectRouteStatements(parsed, typedStmt.List, currentPrefixes)...)
		case *ast.IfStmt:
			registrations = append(registrations, inspectIfStatement(parsed, typedStmt, currentPrefixes)...)
		case *ast.ForStmt:
			registrations = append(registrations, inspectRouteStatements(parsed, typedStmt.Body.List, currentPrefixes)...)
		case *ast.RangeStmt:
			registrations = append(registrations, inspectRouteStatements(parsed, typedStmt.Body.List, currentPrefixes)...)
		}
	}

	return registrations
}

func inspectIfStatement(parsed *parsedGoFile, ifStmt *ast.IfStmt, prefixes map[string]string) []routeRegistration {
	registrations := make([]routeRegistration, 0)
	currentPrefixes := clonePrefixes(prefixes)

	if ifStmt.Init != nil {
		switch init := ifStmt.Init.(type) {
		case *ast.AssignStmt:
			for index, rhs := range init.Rhs {
				if prefix, ok := resolveGroupPrefix(rhs, currentPrefixes); ok && index < len(init.Lhs) {
					if ident, ok := init.Lhs[index].(*ast.Ident); ok {
						currentPrefixes[ident.Name] = prefix
					}
				}
			}
		}
	}

	registrations = append(registrations, inspectRouteStatements(parsed, ifStmt.Body.List, currentPrefixes)...)
	if ifStmt.Else != nil {
		switch typedElse := ifStmt.Else.(type) {
		case *ast.BlockStmt:
			registrations = append(registrations, inspectRouteStatements(parsed, typedElse.List, currentPrefixes)...)
		case *ast.IfStmt:
			registrations = append(registrations, inspectIfStatement(parsed, typedElse, currentPrefixes)...)
		}
	}
	return registrations
}

func extractRegistrationsFromExpr(parsed *parsedGoFile, expr ast.Expr, prefixes map[string]string) []routeRegistration {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}

	if registration, ok := buildRouteRegistration(parsed, call, prefixes); ok {
		return []routeRegistration{registration}
	}

	return nil
}

func buildRouteRegistration(parsed *parsedGoFile, call *ast.CallExpr, prefixes map[string]string) (routeRegistration, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !isGinRouteMethod(selector.Sel.Name) || len(call.Args) == 0 {
		return routeRegistration{}, false
	}

	pathLiteral, ok := stringLiteral(call.Args[0])
	if !ok {
		return routeRegistration{}, false
	}

	fullPath, ok := resolveCallPath(selector.X, pathLiteral, prefixes)
	if !ok {
		fullPath = normalizeRoutePath(pathLiteral)
	}

	handlerTargets := uniqueStrings(extractHandlerTargets(call.Args[1:]))
	primaryHandler := ""
	if len(handlerTargets) > 0 {
		primaryHandler = handlerTargets[len(handlerTargets)-1]
	}

	return routeRegistration{
		Method:         strings.ToUpper(selector.Sel.Name),
		Path:           normalizeRoutePath(pathLiteral),
		FullPath:       fullPath,
		HandlerTargets: handlerTargets,
		PrimaryHandler: primaryHandler,
		Range:          sourceRange(parsed.FileSet, call.Pos(), call.End()),
	}, true
}

func resolveCallPath(receiver ast.Expr, routePath string, prefixes map[string]string) (string, bool) {
	switch typedReceiver := receiver.(type) {
	case *ast.Ident:
		prefix, ok := prefixes[typedReceiver.Name]
		if !ok {
			return normalizeRoutePath(routePath), true
		}
		return joinRoutePath(prefix, routePath), true
	case *ast.CallExpr:
		prefix, ok := resolveGroupPrefix(typedReceiver, prefixes)
		if !ok {
			return "", false
		}
		return joinRoutePath(prefix, routePath), true
	case *ast.SelectorExpr:
		return resolveCallPath(typedReceiver.X, routePath, prefixes)
	default:
		return "", false
	}
}

func resolveGroupPrefix(expr ast.Expr, prefixes map[string]string) (string, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return "", false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil || selector.Sel.Name != "Group" {
		return "", false
	}

	groupPath := ""
	if len(call.Args) > 0 {
		literal, ok := stringLiteral(call.Args[0])
		if !ok {
			return "", false
		}
		groupPath = literal
	}

	basePrefix, ok := resolveReceiverPrefix(selector.X, prefixes)
	if !ok {
		basePrefix = ""
	}

	return joinRoutePath(basePrefix, groupPath), true
}

func resolveReceiverPrefix(expr ast.Expr, prefixes map[string]string) (string, bool) {
	switch typedExpr := expr.(type) {
	case *ast.Ident:
		prefix, ok := prefixes[typedExpr.Name]
		if !ok {
			return "", false
		}
		return prefix, true
	case *ast.CallExpr:
		return resolveGroupPrefix(typedExpr, prefixes)
	case *ast.SelectorExpr:
		return resolveReceiverPrefix(typedExpr.X, prefixes)
	default:
		return "", false
	}
}

func extractHandlerTargets(arguments []ast.Expr) []string {
	targets := make([]string, 0)
	for _, argument := range arguments {
		targets = append(targets, extractHandlerTargetsFromExpr(argument)...)
	}
	return uniqueStrings(targets)
}

func extractHandlerTargetsFromExpr(expr ast.Expr) []string {
	switch typedExpr := expr.(type) {
	case *ast.SelectorExpr:
		target := selectorChain(typedExpr)
		if shouldIgnoreTarget(target) {
			return nil
		}
		return []string{target}
	case *ast.Ident:
		if typedExpr.Name == "" {
			return nil
		}
		return []string{typedExpr.Name}
	case *ast.CallExpr:
		targets := make([]string, 0)
		for _, argument := range typedExpr.Args {
			targets = append(targets, extractHandlerTargetsFromExpr(argument)...)
		}
		return uniqueStrings(targets)
	case *ast.FuncLit:
		targets := make([]string, 0)
		ast.Inspect(typedExpr.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				target := selectorChain(fun)
				if shouldIgnoreTarget(target) {
					return true
				}
				targets = append(targets, target)
			case *ast.Ident:
				if fun.Name != "" {
					targets = append(targets, fun.Name)
				}
			}
			return true
		})
		return uniqueStrings(targets)
	default:
		return nil
	}
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

func discoverDerivedHandlers(cache *parserCache, routeNodes, existingNodes []model.ChainNode, repoSide string) ([]model.ChainNode, error) {
	handlerFiles, err := collectHandlerFiles(cache.repoRoot)
	if err != nil {
		return nil, err
	}

	targetNames := collectHandlerTargetNames(routeNodes)
	if len(targetNames) == 0 || len(handlerFiles) == 0 {
		return nil, nil
	}

	existingKeys := existingNodeKeys(existingNodes)
	derived := make([]model.ChainNode, 0)
	for _, handlerFile := range handlerFiles {
		parsed, err := cache.parse(handlerFile)
		if err != nil {
			return nil, err
		}

		for _, decl := range parsed.File.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Name == nil {
				continue
			}

			if _, ok := targetNames[funcDecl.Name.Name]; !ok {
				continue
			}

			key := parsed.RelativePath + "#" + funcDecl.Name.Name
			if _, exists := existingKeys[key]; exists {
				continue
			}

			derived = append(derived, model.ChainNode{
				Kind:       model.NodeKindHandler,
				Language:   "go",
				FilePath:   parsed.RelativePath,
				SymbolName: funcDecl.Name.Name,
				Range:      sourceRange(parsed.FileSet, funcDecl.Pos(), funcDecl.End()),
				Metadata: map[string]any{
					"repoSide":         repoSide,
					"adapter":          "gox",
					"exists":           true,
					"extracted":        true,
					"source":           "go-ast-derived-handler",
					"derivedFromRoute": true,
				},
			})
			existingKeys[key] = struct{}{}
		}
	}

	return derived, nil
}

func collectRouteFiles(root string) ([]string, error) {
	files := make([]string, 0)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".forktool", ".cache", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(strings.ToLower(entry.Name()), "_test.go") {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(relativePath)
		if !isRouteCandidateFile(normalized) {
			return nil
		}

		files = append(files, normalized)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk route files: %w", err)
	}

	slices.Sort(files)
	return files, nil
}

func collectHandlerFiles(root string) ([]string, error) {
	files := make([]string, 0)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".forktool", ".cache", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(strings.ToLower(entry.Name()), "_test.go") {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(relativePath)
		if !strings.Contains(strings.ToLower(normalized), "/handler/") {
			return nil
		}

		files = append(files, normalized)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk handler files: %w", err)
	}

	slices.Sort(files)
	return files, nil
}

func isRouteCandidateFile(filePath string) bool {
	lowerFile := strings.ToLower(filepath.ToSlash(filePath))
	switch {
	case strings.HasPrefix(lowerFile, ".cache/"):
		return false
	case strings.Contains(lowerFile, "/pkg/mod/"):
		return false
	case strings.Contains(lowerFile, "/routes/"):
		return true
	case strings.HasSuffix(lowerFile, "/router.go"):
		return true
	case strings.HasSuffix(lowerFile, "/setup/handler.go"):
		return true
	case strings.HasSuffix(lowerFile, "/server/router.go"):
		return true
	default:
		return false
	}
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

func routePatternMatches(pathPattern, candidatePath string) bool {
	pathPattern = normalizeRoutePath(pathPattern)
	candidatePath = normalizeRoutePath(candidatePath)
	if pathPattern == "" || candidatePath == "" {
		return false
	}

	if pathPattern == candidatePath {
		return true
	}

	patternPrefix := wildcardPrefix(pathPattern)
	candidatePrefix := wildcardPrefix(candidatePath)

	switch {
	case patternPrefix != "" && strings.HasPrefix(candidatePath, patternPrefix):
		return true
	case candidatePrefix != "" && strings.HasPrefix(pathPattern, candidatePrefix):
		return true
	default:
		return false
	}
}

func wildcardPrefix(value string) string {
	value = normalizeRoutePath(value)
	if value == "" {
		return ""
	}

	index := len(value)
	if wildcardIndex := strings.IndexAny(value, "*:"); wildcardIndex >= 0 {
		index = wildcardIndex
	}
	if index == len(value) {
		return ""
	}

	prefix := strings.TrimSuffix(value[:index], "/")
	if prefix == "" {
		return "/"
	}
	return prefix
}

func isGinRouteMethod(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "ANY", "MATCH", "HANDLE":
		return true
	default:
		return false
	}
}

func selectorChain(selector *ast.SelectorExpr) string {
	parts := make([]string, 0, 4)
	current := ast.Expr(selector)
	for current != nil {
		switch typed := current.(type) {
		case *ast.SelectorExpr:
			parts = append(parts, typed.Sel.Name)
			current = typed.X
		case *ast.Ident:
			parts = append(parts, typed.Name)
			current = nil
		default:
			current = nil
		}
	}
	slices.Reverse(parts)
	return strings.Join(parts, ".")
}

func shouldIgnoreTarget(target string) bool {
	return strings.HasPrefix(target, "c.") || strings.HasPrefix(target, "gin.")
}

func functionParamNames(funcDecl *ast.FuncDecl) []string {
	if funcDecl.Type == nil || funcDecl.Type.Params == nil {
		return nil
	}

	names := make([]string, 0)
	for _, field := range funcDecl.Type.Params.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

func clonePrefixes(prefixes map[string]string) map[string]string {
	cloned := make(map[string]string, len(prefixes))
	for key, value := range prefixes {
		cloned[key] = value
	}
	return cloned
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
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

func collectHandlerTargetNames(routeNodes []model.ChainNode) map[string]struct{} {
	targets := make(map[string]struct{})
	for _, node := range routeNodes {
		if node.Kind != model.NodeKindRoute {
			continue
		}

		handlerTargets, ok := node.Metadata["handlerTargets"].([]string)
		if !ok {
			continue
		}

		for _, target := range handlerTargets {
			if !looksLikeHandlerTarget(target) {
				continue
			}
			name := targetLeaf(target)
			if name == "" {
				continue
			}
			targets[name] = struct{}{}
		}
	}
	return targets
}

func existingNodeKeys(nodes []model.ChainNode) map[string]struct{} {
	keys := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.FilePath) == "" || strings.TrimSpace(node.SymbolName) == "" {
			continue
		}
		keys[node.FilePath+"#"+node.SymbolName] = struct{}{}
	}
	return keys
}

func looksLikeHandlerTarget(target string) bool {
	return strings.Contains(target, "Gateway.") || strings.Contains(strings.ToLower(target), "handler")
}

func targetLeaf(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	parts := strings.Split(target, ".")
	return parts[len(parts)-1]
}

func joinRoutePath(prefix, route string) string {
	prefix = normalizeRoutePath(prefix)
	route = strings.TrimSpace(route)
	if route == "" {
		return prefix
	}
	if prefix == "" {
		return normalizeRoutePath(route)
	}
	return normalizeRoutePath(strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(route, "/"))
}

func normalizeRoutePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	for strings.Contains(value, "//") {
		value = strings.ReplaceAll(value, "//", "/")
	}
	if len(value) > 1 {
		value = strings.TrimSuffix(value, "/")
	}
	return value
}

func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}

	unquoted, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return unquoted, true
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
