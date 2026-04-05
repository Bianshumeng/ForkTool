package manifest

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"forktool/pkg/model"
)

var (
	validRiskLevels = []string{"critical", "high", "medium", "low"}
	validLanguages  = []string{"go", "ts", "typescript", "vue", "sql", "yaml", "json", "markdown"}
	validFormats    = []string{"md", "json"}
)

func Load(path string) (model.FeatureManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return model.FeatureManifest{}, fmt.Errorf("read manifest: %w", err)
	}

	var manifest model.FeatureManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return model.FeatureManifest{}, fmt.Errorf("parse manifest: %w", err)
	}

	return manifest, nil
}

func LoadAndValidate(path string) (model.FeatureManifest, model.ManifestValidationResult, error) {
	manifest, err := Load(path)
	if err != nil {
		return model.FeatureManifest{}, model.ManifestValidationResult{
			Path:   path,
			Valid:  false,
			Errors: []string{err.Error()},
		}, err
	}

	result := Validate(path, manifest)
	if !result.Valid {
		return manifest, result, fmt.Errorf("manifest validation failed")
	}

	return manifest, result, nil
}

func Validate(path string, manifest model.FeatureManifest) model.ManifestValidationResult {
	result := model.ManifestValidationResult{
		Path:         path,
		RepoKind:     manifest.RepoKind,
		Version:      manifest.Version,
		FeatureCount: len(manifest.Features),
		Valid:        true,
	}

	var errors []string

	if manifest.Version <= 0 {
		errors = append(errors, "version must be greater than 0")
	}

	if strings.TrimSpace(manifest.RepoKind) == "" {
		errors = append(errors, "repoKind is required")
	}

	for _, format := range manifest.Defaults.ReportFormats {
		if !slices.Contains(validFormats, strings.ToLower(strings.TrimSpace(format))) {
			errors = append(errors, fmt.Sprintf("defaults.reportFormats contains unsupported format %q", format))
		}
	}

	if len(manifest.Features) == 0 {
		errors = append(errors, "at least one feature is required")
	}

	seenIDs := make(map[string]struct{}, len(manifest.Features))
	for index, feature := range manifest.Features {
		prefix := fmt.Sprintf("features[%d]", index)
		if strings.TrimSpace(feature.ID) == "" {
			errors = append(errors, prefix+".id is required")
		} else {
			if _, ok := seenIDs[feature.ID]; ok {
				errors = append(errors, fmt.Sprintf("duplicate feature id %q", feature.ID))
			}
			seenIDs[feature.ID] = struct{}{}
		}

		if strings.TrimSpace(feature.Name) == "" {
			errors = append(errors, prefix+".name is required")
		}

		if !slices.Contains(validRiskLevels, strings.ToLower(strings.TrimSpace(feature.RiskLevel))) {
			errors = append(errors, fmt.Sprintf("%s.riskLevel must be one of %v", prefix, validRiskLevels))
		}

		if len(feature.Languages) == 0 {
			errors = append(errors, prefix+".languages must contain at least one language")
		}

		for _, language := range feature.Languages {
			if !slices.Contains(validLanguages, strings.ToLower(strings.TrimSpace(language))) {
				errors = append(errors, fmt.Sprintf("%s.languages contains unsupported language %q", prefix, language))
			}
		}

		if len(feature.Chain.Routes) == 0 && len(feature.Chain.Symbols) == 0 && len(feature.AllTests()) == 0 {
			errors = append(errors, prefix+" must define at least one route, symbol, or test")
		}

		for routeIndex, route := range feature.Chain.Routes {
			if strings.TrimSpace(route.PathPattern) == "" {
				errors = append(errors, fmt.Sprintf("%s.chain.routes[%d].pathPattern is required", prefix, routeIndex))
			}
		}

		for symbolIndex, symbol := range feature.Chain.Symbols {
			if strings.TrimSpace(symbol.File) == "" {
				errors = append(errors, fmt.Sprintf("%s.chain.symbols[%d].file is required", prefix, symbolIndex))
			}
			if len(symbol.Functions) == 0 {
				errors = append(errors, fmt.Sprintf("%s.chain.symbols[%d].functions must contain at least one function", prefix, symbolIndex))
			}
		}

		if len(feature.SemanticRules) == 0 {
			errors = append(errors, prefix+".semanticRules must contain at least one rule id")
		}

		if strings.TrimSpace(feature.Decisions.Default) == "" {
			errors = append(errors, prefix+".decisions.default is required")
		}
	}

	result.Errors = errors
	result.Valid = len(errors) == 0
	return result
}

func FindFeature(manifest model.FeatureManifest, featureID string) (model.ManifestFeature, bool) {
	for _, feature := range manifest.Features {
		if feature.ID == featureID {
			return feature, true
		}
	}
	return model.ManifestFeature{}, false
}
