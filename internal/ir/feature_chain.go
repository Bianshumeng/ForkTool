package ir

import "forktool/pkg/model"

func NewFeatureChain(feature model.ManifestFeature, localNodes, officialNodes []model.ChainNode, decisionHints []model.DecisionHint) model.FeatureChain {
	return model.FeatureChain{
		ID:              feature.ID,
		Name:            feature.Name,
		RiskLevel:       feature.RiskLevel,
		Languages:       feature.Languages,
		LocalNodes:      localNodes,
		OfficialNodes:   officialNodes,
		SemanticRules:   feature.SemanticRules,
		DefaultDecision: feature.Decisions.Default,
		DecisionHints:   decisionHints,
	}
}
