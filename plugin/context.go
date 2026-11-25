package plugin

import (
	"context"
	"fmt"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// ContextFunc builds the PluginConfig sent with one invocation.
type ContextFunc func(ctx context.Context) (PluginConfig, error)

// EngagementContext returns a ContextFunc that asks active for the current
// engagement on every call and loads its name and scope from store.
func EngagementContext(store tools.Store, active func() (taskID, explorationID int64), dataDir string, proxy *Proxy) ContextFunc {
	return func(ctx context.Context) (PluginConfig, error) {
		taskID, explorationID := active()
		cfg := PluginConfig{
			Engagement: Engagement{ID: taskID, ExplorationID: explorationID},
			InScope:    []tools.ScopeRule{},
			DataDir:    dataDir,
			Proxy:      proxy,
		}
		if taskID <= 0 {
			return cfg, nil
		}
		eng, err := store.GetEngagement(ctx, taskID)
		if err != nil {
			return PluginConfig{}, fmt.Errorf("loading engagement %d: %w", taskID, err)
		}
		if eng != nil {
			cfg.Engagement.Name = eng.Goal
		}
		rules, err := store.ListScopeRules(ctx, taskID)
		if err != nil {
			return PluginConfig{}, fmt.Errorf("loading scope for engagement %d: %w", taskID, err)
		}
		if len(rules) > 0 {
			cfg.InScope = rules
		}
		return cfg, nil
	}
}
