package plugin

import "encoding/json"

// Manifest describes a plugin and the tools it provides.
// A plugin binary prints this as JSON to stdout when called with --manifest.
type Manifest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Tools       []ManifestTool `json:"tools"`
}

// ManifestTool describes a single tool exposed by a plugin.
type ManifestTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

// Invocation is the JSON sent to a plugin on stdin when a tool is called.
type Invocation struct {
	Tool   string          `json:"tool"`
	Input  json.RawMessage `json:"input"`
	Config PluginConfig    `json:"config"`
}

// PluginConfig carries server-side context a plugin might need.
type PluginConfig struct{}

// Result is the JSON a plugin writes to stdout after handling an invocation.
type Result struct {
	Content []ResultContent `json:"content"`
	IsError bool            `json:"isError,omitempty"`
}

// ResultContent is one piece of output content from a plugin.
type ResultContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
