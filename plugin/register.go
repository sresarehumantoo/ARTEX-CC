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
// If dir does not exist, it returns nil. contextFn, when non-nil, supplies
// the invocation config at call time.
func DiscoverAndRegister(registry *tools.Registry, dir string, timeout time.Duration, contextFn ContextFunc) error {
	ctx := context.Background()
	plugins, err := Discover(ctx, dir)
	if err != nil {
		return fmt.Errorf("plugin discovery: %w", err)
	}

	for _, p := range plugins {
		registerPlugin(registry, p, timeout, contextFn)
	}
	return nil
}

func registerPlugin(registry *tools.Registry, p discovered, timeout time.Duration, contextFn ContextFunc) {
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
			Run:         makeHandler(registry, binPath, pluginToolName, timeout, contextFn),
		})
		slog.Debug("registered plugin tool", "tool", toolName, "plugin", p.Manifest.Name)
	}
}

func makeHandler(registry *tools.Registry, binPath, toolName string, timeout time.Duration, contextFn ContextFunc) func(context.Context, json.RawMessage) (tools.Result, error) {
	return func(ctx context.Context, input json.RawMessage) (tools.Result, error) {
		inv := Invocation{
			Tool:  toolName,
			Input: input,
		}
		target := registry
		if contextFn != nil {
			cfg, reg, err := contextFn(ctx)
			if err != nil {
				return tools.Result{}, fmt.Errorf("building plugin context: %w", err)
			}
			inv.Config = cfg
			target = reg
		}
		result, err := Exec(ctx, binPath, inv, timeout)
		if err != nil {
			return tools.Result{}, err
		}

		var parts []string
		for _, c := range result.Content {
			parts = append(parts, c.Text)
		}
		if !result.IsError {
			pctx, cancel := context.WithTimeout(ctx, PersistTimeout)
			defer cancel()
			if summary := persistResults(pctx, target, result); summary != "" {
				parts = append(parts, summary)
			}
		}
		return tools.Result{
			Content: strings.Join(parts, "\n"),
			IsError: result.IsError,
		}, nil
	}
}

// Limits on what one plugin result may write to the graph. A result over
// MaxResultItems is rejected whole rather than truncated, so the graph never
// holds an arbitrary prefix of it.
const (
	MaxResultItems  = 500
	PersistTimeout  = time.Minute
	MaxProblemLines = 20
)

// persistResults writes a plugin's structured output through the same tools
// the model would call, so scope checks and validation apply unchanged.
// Items go one at a time so a bad item never blocks the rest.
func persistResults(ctx context.Context, registry *tools.Registry, r Result) string {
	total := len(r.Assets) + len(r.Facts) + len(r.Findings)
	if total == 0 {
		return ""
	}
	if total > MaxResultItems {
		return fmt.Sprintf("persisted nothing: result has %d structured items, max %d", total, MaxResultItems)
	}

	var assets, facts, findings, outOfScope, attempted int
	var notes []string

	for i, a := range r.Assets {
		if ctx.Err() != nil {
			break
		}
		attempted++
		res, err := runTool(ctx, registry, "insert_assets", map[string]any{"assets": []json.RawMessage{a}})
		if err != nil {
			notes = append(notes, fmt.Sprintf("failed asset %d: %v", i, err))
			continue
		}
		var out struct {
			Results []json.RawMessage `json:"results"`
			Errors  []struct {
				Error string `json:"error"`
			} `json:"errors"`
		}
		if res.IsError || json.Unmarshal([]byte(res.Content), &out) != nil {
			notes = append(notes, fmt.Sprintf("failed asset %d: %s", i, res.Content))
			continue
		}
		switch {
		case len(out.Results) == 1:
			assets++
		case len(out.Errors) > 0 && strings.HasPrefix(out.Errors[0].Error, "out of scope"):
			outOfScope++
			notes = append(notes, fmt.Sprintf("skipped asset %d: %s", i, out.Errors[0].Error))
		case len(out.Errors) > 0:
			notes = append(notes, fmt.Sprintf("failed asset %d: %s", i, out.Errors[0].Error))
		}
	}

	for i, f := range r.Facts {
		if ctx.Err() != nil {
			break
		}
		attempted++
		res, err := runTool(ctx, registry, "record_fact", map[string]any{"facts": []json.RawMessage{f}})
		switch {
		case err != nil:
			notes = append(notes, fmt.Sprintf("failed fact %d: %v", i, err))
		case res.IsError:
			notes = append(notes, fmt.Sprintf("failed fact %d: %s", i, res.Content))
		default:
			facts++
		}
	}

	for i, f := range r.Findings {
		if ctx.Err() != nil {
			break
		}
		attempted++
		res, err := runTool(ctx, registry, "report_finding", f)
		switch {
		case err != nil:
			notes = append(notes, fmt.Sprintf("failed finding %d: %v", i, err))
		case res.IsError:
			notes = append(notes, fmt.Sprintf("failed finding %d: %s", i, res.Content))
		default:
			findings++
		}
	}

	summary := fmt.Sprintf("inserted %s, %s, %s", plural(assets, "asset"), plural(facts, "fact"), plural(findings, "finding"))
	if outOfScope > 0 {
		summary += fmt.Sprintf("; skipped %d out of scope", outOfScope)
	}
	if attempted < total {
		summary += fmt.Sprintf("; stopped (%v) with %d items not attempted", ctx.Err(), total-attempted)
	}
	if len(notes) > MaxProblemLines {
		notes = append(notes[:MaxProblemLines], fmt.Sprintf("... and %d more problems", len(notes)-MaxProblemLines))
	}
	return strings.Join(append([]string{summary}, notes...), "\n")
}

func runTool(ctx context.Context, registry *tools.Registry, name string, input any) (tools.Result, error) {
	t, ok := registry.Lookup(name)
	if !ok {
		return tools.Result{}, fmt.Errorf("%s is not registered", name)
	}
	b, err := json.Marshal(input)
	if err != nil {
		return tools.Result{}, err
	}
	return t.Run(ctx, b)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
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
