package decision

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"forktool/pkg/model"
)

func Load(path string) (model.DecisionFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return model.DecisionFile{}, fmt.Errorf("read decision file: %w", err)
	}

	var decisionFile model.DecisionFile
	if err := yaml.Unmarshal(content, &decisionFile); err != nil {
		return model.DecisionFile{}, fmt.Errorf("parse decision file: %w", err)
	}

	if decisionFile.Version <= 0 {
		decisionFile.Version = 1
	}

	for index := range decisionFile.Decisions {
		if strings.TrimSpace(decisionFile.Decisions[index].Source) == "" {
			decisionFile.Decisions[index].Source = path
		}
	}

	return decisionFile, nil
}

func FilterForFeature(decisionFile model.DecisionFile, featureID string) []model.DecisionHint {
	filtered := make([]model.DecisionHint, 0)
	for _, hint := range decisionFile.Decisions {
		if hint.FeatureID == featureID {
			filtered = append(filtered, hint)
		}
	}
	return filtered
}
