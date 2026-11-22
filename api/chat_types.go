package api

import "encoding/json"

const (
	ChatStatusIdle    = "idle"
	ChatStatusRunning = "running"
)

const (
	ChatKindUser       = "user"
	ChatKindText       = "text"
	ChatKindThinking   = "thinking"
	ChatKindToolUse    = "tool_use"
	ChatKindToolResult = "tool_result"
	ChatKindResult     = "result"
	ChatKindError      = "error"
)

type Conversation struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Model        string `json:"model,omitempty"`
	Agent        string `json:"agent,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type CreateConversationRequest struct {
	Title        string `json:"title"`
	Model        string `json:"model"`
	Agent        string `json:"agent"`
	SystemPrompt string `json:"system_prompt"`
}

type UpdateConversationRequest struct {
	Title *string `json:"title"`
	Model *string `json:"model"`
	Agent *string `json:"agent"`
}

type SendMessageRequest struct {
	Message string `json:"message"`
}

type ChatUsage struct {
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens int     `json:"cache_creation_tokens,omitempty"`
	CostUSD             float64 `json:"cost_usd,omitempty"`
	DurationMS          int64   `json:"duration_ms,omitempty"`
	NumTurns            int     `json:"num_turns,omitempty"`
}

type ChatEvent struct {
	Seq      int64           `json:"seq"`
	Kind     string          `json:"kind"`
	Text     string          `json:"text,omitempty"`
	ToolName string          `json:"tool_name,omitempty"`
	ToolID   string          `json:"tool_id,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	IsError  bool            `json:"is_error,omitempty"`
	Usage    *ChatUsage      `json:"usage,omitempty"`
	Time     string          `json:"time"`
}
