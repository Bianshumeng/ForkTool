package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"forktool/pkg/model"
)

const defaultSub2APIManifestTemplate = `version: 1
repoKind: sub2api

defaults:
  reportFormats:
    - md
    - json
    - bd
  decisionFile: "./decisions/sub2api.local-decisions.yaml"

features:
  - id: claude-count-tokens
    name: Claude count_tokens 主链
    riskLevel: critical
    owners: [backend]
    languages: [go]
    chain:
      routes:
        - pathPattern: "/v1/messages/count_tokens"
      symbols:
        - file: backend/internal/service/gateway_service.go
          functions:
            - ForwardCountTokens
            - buildCountTokensRequest
      tests:
        - backend/internal/service/gateway_anthropic_apikey_passthrough_test.go
    semanticRules:
      - claude-count-tokens-beta-suffix
      - http-header-wire-casing
      - response-header-filter
    decisions:
      default: official-required
`

type workspaceState struct {
	Config     model.WorkspaceConfig
	ConfigPath string
	RepoRoot   string
}

func defaultConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".forktool", "config.yaml")
}

func resolveConfigPath(workdir, configured string) string {
	if strings.TrimSpace(configured) == "" {
		return defaultConfigPath(workdir)
	}
	if filepath.IsAbs(configured) {
		return configured
	}
	return filepath.Join(workdir, configured)
}

func loadWorkspace(configPath string) (workspaceState, error) {
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return workspaceState{}, fmt.Errorf("resolve config path: %w", err)
	}

	configLoader := viper.New()
	configLoader.SetConfigFile(absoluteConfigPath)
	configLoader.SetConfigType("yaml")
	if err := configLoader.ReadInConfig(); err != nil {
		return workspaceState{}, fmt.Errorf("read config: %w", err)
	}

	var config model.WorkspaceConfig
	if err := configLoader.Unmarshal(&config); err != nil {
		return workspaceState{}, fmt.Errorf("decode config: %w", err)
	}

	return workspaceState{
		Config:     config,
		ConfigPath: absoluteConfigPath,
		RepoRoot:   repoRootFromConfigPath(absoluteConfigPath),
	}, nil
}

func loadWorkspaceIfExists(configPath string) (workspaceState, bool, error) {
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return workspaceState{}, false, fmt.Errorf("resolve config path: %w", err)
	}

	if _, err := os.Stat(absoluteConfigPath); err != nil {
		if os.IsNotExist(err) {
			return workspaceState{}, false, nil
		}
		return workspaceState{}, false, fmt.Errorf("stat config: %w", err)
	}

	workspace, err := loadWorkspace(absoluteConfigPath)
	if err != nil {
		return workspaceState{}, true, err
	}
	return workspace, true, nil
}

func repoRootFromConfigPath(configPath string) string {
	configDir := filepath.Dir(configPath)
	if filepath.Base(configDir) == ".forktool" {
		return filepath.Dir(configDir)
	}
	return configDir
}

func initializeWorkspace(repoRoot, repoKind, toolVersion string, force bool) (workspaceState, error) {
	workspaceDir := filepath.Join(repoRoot, ".forktool")
	for _, directory := range []string{
		workspaceDir,
		filepath.Join(workspaceDir, "cache"),
		filepath.Join(workspaceDir, "runs"),
		filepath.Join(workspaceDir, "baselines"),
		filepath.Join(repoRoot, "decisions"),
		filepath.Join(repoRoot, "manifests"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return workspaceState{}, fmt.Errorf("create %q: %w", directory, err)
		}
	}

	configPath := filepath.Join(workspaceDir, "config.yaml")
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return workspaceState{}, fmt.Errorf("workspace already initialized at %s", filepath.ToSlash(configPath))
		}
	}

	decisionPath := filepath.Join(repoRoot, "decisions", repoKind+".local-decisions.yaml")
	config := model.WorkspaceConfig{
		ToolVersion: toolVersion,
		LocalRepo: model.RepoConfig{
			Path: filepath.ToSlash(repoRoot),
			Kind: "fork",
		},
		OfficialRepo: model.RepoConfig{
			Path: filepath.ToSlash(filepath.Join(repoRoot, "..", "official")),
			Kind: "official",
		},
		Manifest: model.ManifestRef{
			Path: "./manifests/" + repoKind + ".yaml",
		},
		DecisionFile: model.DecisionFileRef{
			Path: "./decisions/" + filepath.Base(decisionPath),
		},
		Output: model.OutputConfig{
			Dir:     "./.forktool/runs",
			Formats: []string{"md", "json"},
		},
	}

	if err := writeYAML(configPath, config); err != nil {
		return workspaceState{}, err
	}

	manifestTemplatePath := filepath.Join(repoRoot, "manifests", repoKind+".yaml")
	if force || !fileExists(manifestTemplatePath) {
		content, err := manifestTemplateContent(repoRoot, repoKind)
		if err != nil {
			return workspaceState{}, fmt.Errorf("write manifest template: %w", err)
		}
		if err := os.WriteFile(manifestTemplatePath, content, 0o644); err != nil {
			return workspaceState{}, fmt.Errorf("write manifest template: %w", err)
		}
	}

	if force || !fileExists(decisionPath) {
		if err := os.WriteFile(decisionPath, []byte("version: 1\ndecisions: []\n"), 0o644); err != nil {
			return workspaceState{}, fmt.Errorf("write decision file: %w", err)
		}
	}

	return workspaceState{
		Config:     config,
		ConfigPath: configPath,
		RepoRoot:   repoRoot,
	}, nil
}

func writeYAML(path string, value any) error {
	payload, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write file %q: %w", path, err)
	}
	return nil
}

func resolveWorkspacePath(repoRoot, configured string) string {
	if strings.TrimSpace(configured) == "" {
		return ""
	}
	if filepath.IsAbs(configured) {
		return filepath.Clean(configured)
	}
	return filepath.Clean(filepath.Join(repoRoot, configured))
}

func prepareRunDirectory(repoRoot string, config model.WorkspaceConfig, explicitOutputDir, runID string) (string, error) {
	if strings.TrimSpace(explicitOutputDir) != "" {
		outputDir := resolveWorkspacePath(repoRoot, explicitOutputDir)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
		return outputDir, nil
	}

	baseDir := config.Output.Dir
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "./.forktool/runs"
	}
	outputDir := filepath.Join(resolveWorkspacePath(repoRoot, baseDir), runID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}
	return outputDir, nil
}

func writeRunContext(outputDir string, context model.RunContext) (string, error) {
	payload, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal run context: %w", err)
	}

	contextPath := filepath.Join(outputDir, "context.json")
	if err := os.WriteFile(contextPath, payload, 0o644); err != nil {
		return "", fmt.Errorf("write run context: %w", err)
	}
	return filepath.ToSlash(contextPath), nil
}

func resolveFormats(explicit []string, config model.WorkspaceConfig) []string {
	if len(explicit) > 0 {
		return explicit
	}
	if len(config.Output.Formats) > 0 {
		return config.Output.Formats
	}
	return []string{"md", "json"}
}

func newRunID(tag, commit string) string {
	suffix := "manual"
	switch {
	case strings.TrimSpace(tag) != "":
		suffix = sanitizeRunSuffix(tag)
	case strings.TrimSpace(commit) != "":
		suffix = sanitizeRunSuffix(shortCommit(commit))
	}
	return time.Now().Format("20060102_150405") + "_" + suffix
}

func sanitizeRunSuffix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "manual"
	}

	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	return builder.String()
}

func shortCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if len(commit) <= 12 {
		return commit
	}
	return commit[:12]
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func manifestTemplateContent(repoRoot, repoKind string) ([]byte, error) {
	exampleManifestPath := filepath.Join(repoRoot, "examples", repoKind+".manifest.example.yaml")
	if content, err := os.ReadFile(exampleManifestPath); err == nil {
		return content, nil
	}

	switch strings.ToLower(strings.TrimSpace(repoKind)) {
	case "sub2api":
		return []byte(defaultSub2APIManifestTemplate), nil
	default:
		return nil, fmt.Errorf("no manifest template available for repo kind %q", repoKind)
	}
}
