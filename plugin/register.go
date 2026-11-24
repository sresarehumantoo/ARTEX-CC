package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// DiscoverAndRegister scans dir for plugins and registers their tools into
// the registry. Each tool is namespaced as pluginname_toolname.
// If dir does not exist, it returns nil.
func DiscoverAndRegister(registry *tools.Registry, dir string, timeout time.Duration) error {
	ctx := context.Background()
	plugins, err := Discover(ctx, dir)
	if err != nil {
		return fmt.Errorf("plugin discovery: %w", err)
	}

	for _, p := range plugins {
		registerPlugin(registry, p, timeout)
	}
	return nil
}

func registerPlugin(registry *tools.Registry, p discovered, timeout time.Duration) {
	for _, mt := range p.Manifest.Tools {
		toolName := p.Manifest.Name + "_" + mt.Name
		schema, err := schemaToMap(mt.Schema)
		if err != nil {
			slog.Warn("skipping plugin tool with bad schema",
				"plugin", p.Manifest.Name, "tool", mt.Name, "err", err)
			continue
		}

		binPath := p.Path
		pluginToolName := mt.Name

		registry.Register(tools.Tool{
			Name:        toolName,
			Description: mt.Description,
			Schema:      schema,
			Run:         makeHandler(binPath, pluginToolName, timeout),
		})
		slog.Debug("registered plugin tool", "tool", toolName, "plugin", p.Manifest.Name)
	}
}

func makeHandler(binPath, toolName string, timeout time.Duration) func(context.Context, json.RawMessage) (tools.Result, error) {
	return func(ctx context.Context, input json.RawMessage) (tools.Result, error) {
		inv := Invocation{
			Tool:  toolName,
			Input: input,
		}
		result, err := Exec(ctx, binPath, inv, timeout)
		if err != nil {
			return tools.Result{}, err
		}

		var parts []string
		for _, c := range result.Content {
			parts = append(parts, c.Text)
		}
		return tools.Result{
			Content: strings.Join(parts, "\n"),
			IsError: result.IsError,
		}, nil
	}
}

func schemaToMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{"type": "object"}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}
