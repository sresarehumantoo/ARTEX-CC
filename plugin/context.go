package plugin

import (
	"context"
	"fmt"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// ContextFunc builds the PluginConfig sent with one invocation and returns the
// registry whose tools persist the result. Both come from one snapshot of the
// active engagement, so the plugin is never told one engagement while its
// results are written to another.
type ContextFunc func(ctx context.Context) (PluginConfig, *tools.Registry, error)

// EngagementContext returns a ContextFunc that asks active for the current
// engagement and its registry on every call and loads the engagement's name
// and scope from store.
func EngagementContext(store tools.Store, active func() (taskID, explorationID int64, registry *tools.Registry), dataDir string, proxy *Proxy) ContextFunc {
	return func(ctx context.Context) (PluginConfig, *tools.Registry, error) {
		taskID, explorationID, registry := active()
		cfg := PluginConfig{
			Engagement: Engagement{ID: taskID, ExplorationID: explorationID},
			InScope:    []tools.ScopeRule{},
			DataDir:    dataDir,
			Proxy:      proxy,
		}
		if taskID <= 0 {
			return cfg, registry, nil
		}
		eng, err := store.GetEngagement(ctx, taskID)
		if err != nil {
			return PluginConfig{}, nil, fmt.Errorf("loading engagement %d: %w", taskID, err)
		}
		if eng != nil {
			cfg.Engagement.Name = eng.Goal
		}
		rules, err := store.ListScopeRules(ctx, taskID)
		if err != nil {
			return PluginConfig{}, nil, fmt.Errorf("loading scope for engagement %d: %w", taskID, err)
		}
		if len(rules) > 0 {
			cfg.InScope = rules
		}
		return cfg, registry, nil
	}
}
