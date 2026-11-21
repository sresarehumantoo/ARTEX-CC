package tools

import (
	"context"
	"encoding/json"
)

type Result struct {
	Content string
	IsError bool
}

type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	ReadOnly    bool
	Run         func(ctx context.Context, input json.RawMessage) (Result, error)
}

type Registry struct {
	tools []Tool
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(t Tool) {
	r.tools = append(r.tools, t)
}

func (r *Registry) Tools() []Tool {
	return r.tools
}

func (r *Registry) Lookup(name string) (Tool, bool) {
	for _, t := range r.tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
