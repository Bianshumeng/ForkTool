package model

type FeatureManifest struct {
	Version         int               `json:"version" yaml:"version"`
	RepoKind        string            `json:"repoKind" yaml:"repoKind"`
	Defaults        ManifestDefaults  `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	DecisionSources []string          `json:"decisionSources,omitempty" yaml:"decisionSources,omitempty"`
	Features        []ManifestFeature `json:"features" yaml:"features"`
}

type ManifestDefaults struct {
	ReportFormats []string `json:"reportFormats,omitempty" yaml:"reportFormats,omitempty"`
	DecisionFile  string   `json:"decisionFile,omitempty" yaml:"decisionFile,omitempty"`
}

type ManifestFeature struct {
	ID            string            `json:"id" yaml:"id"`
	Name          string            `json:"name" yaml:"name"`
	Description   string            `json:"description,omitempty" yaml:"description,omitempty"`
	RiskLevel     string            `json:"riskLevel" yaml:"riskLevel"`
	Owners        []string          `json:"owners,omitempty" yaml:"owners,omitempty"`
	Languages     []string          `json:"languages" yaml:"languages"`
	Chain         ManifestChain     `json:"chain" yaml:"chain"`
	SemanticRules []string          `json:"semanticRules" yaml:"semanticRules"`
	Tests         []string          `json:"tests,omitempty" yaml:"tests,omitempty"`
	Decisions     ManifestDecisions `json:"decisions" yaml:"decisions"`
	Notes         []string          `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ManifestChain struct {
	Routes  []ManifestRoute  `json:"routes,omitempty" yaml:"routes,omitempty"`
	Symbols []ManifestSymbol `json:"symbols,omitempty" yaml:"symbols,omitempty"`
	Tests   []string         `json:"tests,omitempty" yaml:"tests,omitempty"`
}

type ManifestRoute struct {
	PathPattern string `json:"pathPattern" yaml:"pathPattern"`
}

type ManifestSymbol struct {
	File      string   `json:"file" yaml:"file"`
	Functions []string `json:"functions" yaml:"functions"`
}

type ManifestDecisions struct {
	Default           string   `json:"default" yaml:"default"`
	AllowedLocalHooks []string `json:"allowedLocalHooks,omitempty" yaml:"allowedLocalHooks,omitempty"`
}

func (f ManifestFeature) AllTests() []string {
	merged := make([]string, 0, len(f.Tests)+len(f.Chain.Tests))
	seen := make(map[string]struct{}, len(f.Tests)+len(f.Chain.Tests))

	for _, testPath := range f.Chain.Tests {
		if testPath == "" {
			continue
		}
		if _, ok := seen[testPath]; ok {
			continue
		}
		seen[testPath] = struct{}{}
		merged = append(merged, testPath)
	}

	for _, testPath := range f.Tests {
		if testPath == "" {
			continue
		}
		if _, ok := seen[testPath]; ok {
			continue
		}
		seen[testPath] = struct{}{}
		merged = append(merged, testPath)
	}

	return merged
}
