package discovery

import (
	"context"
	"strings"

	"forktool/pkg/model"
)

type Adapter interface {
	Name() string
	SupportsLanguage(lang string) bool
	Discover(ctx context.Context, req DiscoverRequest) ([]model.ChainNode, error)
}

type DiscoverRequest struct {
	RepoRoot  string
	Feature   model.ManifestFeature
	RepoSide  string
	FileGlobs []string
}

type Manager struct {
	adapters []Adapter
}

func NewManager(adapters ...Adapter) *Manager {
	return &Manager{adapters: adapters}
}

func (m *Manager) Discover(ctx context.Context, req DiscoverRequest) ([]model.ChainNode, error) {
	if m == nil {
		return nil, nil
	}

	collected := make([]model.ChainNode, 0)
	for _, language := range req.Feature.Languages {
		language = strings.ToLower(strings.TrimSpace(language))
		for _, adapter := range m.adapters {
			if !adapter.SupportsLanguage(language) {
				continue
			}

			nodes, err := adapter.Discover(ctx, req)
			if err != nil {
				return nil, err
			}
			collected = append(collected, nodes...)
		}
	}

	return collected, nil
}
