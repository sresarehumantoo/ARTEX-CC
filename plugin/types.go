package plugin

import (
	"encoding/json"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

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

// PluginConfig carries the engagement context a plugin runs in. It is built
// for every invocation, so it follows switch_engagement.
type PluginConfig struct {
	Engagement Engagement        `json:"engagement"`
	InScope    []tools.ScopeRule `json:"in_scope"`
	DataDir    string            `json:"data_dir,omitempty"`
	Proxy      *Proxy            `json:"proxy,omitempty"`
}

// Engagement identifies the active engagement. Name is the engagement goal.
type Engagement struct {
	ID            int64  `json:"id"`
	ExplorationID int64  `json:"exploration_id"`
	Name          string `json:"name"`
}

// Proxy is the recording proxy a plugin can route HTTP(S) traffic through.
// CACert is the PEM file to trust for HTTPS interception.
type Proxy struct {
	URL    string `json:"url"`
	CACert string `json:"ca_cert"`
}

// Result is the JSON a plugin writes to stdout after handling an invocation.
// Assets, Facts and Findings are persisted after a successful run. Each
// element has the same shape as one item accepted by insert_assets
// (assets[i]), record_fact (facts[i]) and report_finding respectively.
type Result struct {
	Content  []ResultContent   `json:"content"`
	IsError  bool              `json:"isError,omitempty"`
	Assets   []json.RawMessage `json:"assets,omitempty"`
	Facts    []json.RawMessage `json:"facts,omitempty"`
	Findings []json.RawMessage `json:"findings,omitempty"`
}

// ResultContent is one piece of output content from a plugin.
type ResultContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
