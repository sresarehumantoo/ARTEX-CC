package tools

import (
	"context"
	"encoding/json"
)

// Result is the return value of an MCP tool invocation.
type Result struct {
	Content string
	IsError bool
}

// Tool categories, in the order the tool catalog presents them.
const (
	CategoryExploration = "Exploration graph"
	CategoryAssets      = "Assets and scope"
	CategoryFindings    = "Findings"
	CategoryCredentials = "Credentials"
	CategoryEvidence    = "Evidence"
	CategoryEnrichment  = "Enrichment and ingestion"
	CategoryEngagement  = "Engagement management"
	CategoryActivity    = "Activity"
	CategoryReporting   = "Reporting and export"
	CategoryTraffic     = "Traffic"
)

// Categories lists every tool category in catalog order.
var Categories = []string{
	CategoryExploration,
	CategoryAssets,
	CategoryFindings,
	CategoryCredentials,
	CategoryEvidence,
	CategoryEnrichment,
	CategoryEngagement,
	CategoryActivity,
	CategoryReporting,
	CategoryTraffic,
}

// Tool is an MCP tool definition with its JSON Schema and handler function.
type Tool struct {
	Name        string
	Description string
	Category    string
	Schema      map[string]any
	ReadOnly    bool
	Run         func(ctx context.Context, input json.RawMessage) (Result, error)
}

// Registry is an ordered collection of registered MCP tools.
type Registry struct {
	tools []Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools = append(r.tools, t)
}

// Tools returns all registered tools in registration order.
func (r *Registry) Tools() []Tool {
	return r.tools
}

// Lookup finds a tool by name, returning false if not found.
func (r *Registry) Lookup(name string) (Tool, bool) {
	for _, t := range r.tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
