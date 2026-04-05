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
	invoked := make(map[string]struct{}, len(m.adapters))
	for _, adapter := range m.adapters {
		if _, ok := invoked[adapter.Name()]; ok {
			continue
		}

		supported := false
		for _, language := range req.Feature.Languages {
			language = strings.ToLower(strings.TrimSpace(language))
			if adapter.SupportsLanguage(language) {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}

		nodes, err := adapter.Discover(ctx, req)
		if err != nil {
			return nil, err
		}
		collected = append(collected, nodes...)
		invoked[adapter.Name()] = struct{}{}
	}

	return collected, nil
}
