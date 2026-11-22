package store

import (
	"encoding/json"
	"testing"
)

func TestMatchesRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		rule      InterceptRule
		toolName  string
		toolInput json.RawMessage
		want      bool
	}{
		{
			"string match on tool_name",
			InterceptRule{MatchTarget: "tool_name", MatchType: "string", Pattern: "shell"},
			"run_shell_command",
			json.RawMessage(`{}`),
			true,
		},
		{
			"string match case insensitive",
			InterceptRule{MatchTarget: "tool_name", MatchType: "string", Pattern: "SHELL"},
			"run_shell_command",
			json.RawMessage(`{}`),
			true,
		},
		{
			"string no match",
			InterceptRule{MatchTarget: "tool_name", MatchType: "string", Pattern: "database"},
			"run_shell_command",
			json.RawMessage(`{}`),
			false,
		},
		{
			"regex match on tool_name",
			InterceptRule{MatchTarget: "tool_name", MatchType: "regex", Pattern: "^run_.*"},
			"run_shell_command",
			json.RawMessage(`{}`),
			true,
		},
		{
			"regex no match",
			InterceptRule{MatchTarget: "tool_name", MatchType: "regex", Pattern: "^delete_.*"},
			"run_shell_command",
			json.RawMessage(`{}`),
			false,
		},
		{
			"string match on tool_input",
			InterceptRule{MatchTarget: "tool_input", MatchType: "string", Pattern: "password"},
			"some_tool",
			json.RawMessage(`{"field":"password123"}`),
			true,
		},
		{
			"regex match on tool_input",
			InterceptRule{MatchTarget: "tool_input", MatchType: "regex", Pattern: `"command"\s*:\s*"rm\s`},
			"shell",
			json.RawMessage(`{"command":"rm -rf /"}`),
			true,
		},
		{
			"unknown match_target",
			InterceptRule{MatchTarget: "unknown", MatchType: "string", Pattern: "x"},
			"test",
			json.RawMessage(`{}`),
			false,
		},
		{
			"unknown match_type",
			InterceptRule{MatchTarget: "tool_name", MatchType: "glob", Pattern: "*"},
			"test",
			json.RawMessage(`{}`),
			false,
		},
		{
			"invalid regex",
			InterceptRule{MatchTarget: "tool_name", MatchType: "regex", Pattern: "[invalid"},
			"test",
			json.RawMessage(`{}`),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesRule(tt.rule, tt.toolName, tt.toolInput)
			if got != tt.want {
				t.Errorf("matchesRule() = %v, want %v", got, tt.want)
			}
		})
	}
}
