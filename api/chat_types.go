package api

import "time"

type ConversationStatus string

const (
	ConversationIdle    ConversationStatus = "idle"
	ConversationRunning ConversationStatus = "running"
	ConversationError   ConversationStatus = "error"
)

type Conversation struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Model     string             `json:"model"`
	Agent     string             `json:"agent"`
	Status    ConversationStatus `json:"status"`
	SessionID string             `json:"session_id"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type ChatEventKind string

const (
	ChatEventText       ChatEventKind = "text"
	ChatEventThinking   ChatEventKind = "thinking"
	ChatEventToolUse    ChatEventKind = "tool_use"
	ChatEventToolResult ChatEventKind = "tool_result"
	ChatEventResult     ChatEventKind = "result"
	ChatEventError      ChatEventKind = "error"
	ChatEventStatus     ChatEventKind = "status"
)

type ChatEvent struct {
	Seq            int64              `json:"seq"`
	Timestamp      time.Time          `json:"ts"`
	ConversationID string             `json:"conversation_id"`
	Role           string             `json:"role"`
	Kind           ChatEventKind      `json:"kind"`
	Text           string             `json:"text,omitempty"`
	Tool           *ToolCall          `json:"tool,omitempty"`
	ToolResult     *ToolResult        `json:"tool_result,omitempty"`
	Usage          *TokenUsage        `json:"usage,omitempty"`
	Status         ConversationStatus `json:"status,omitempty"`
}

type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

type TokenUsage struct {
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CostUSD             float64 `json:"cost_usd,omitempty"`
}

type ChatAttachment struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type ChatMention struct {
	Type  string `json:"type"`
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

type SendMessageRequest struct {
	Text        string           `json:"text"`
	Attachments []ChatAttachment `json:"attachments,omitempty"`
	Mentions    []ChatMention    `json:"mentions,omitempty"`
}

type CreateConversationRequest struct {
	Title string `json:"title,omitempty"`
	Model string `json:"model,omitempty"`
	Agent string `json:"agent,omitempty"`
}

type ClaudeStatus struct {
	BinaryFound bool   `json:"binary_found"`
	Version     string `json:"version"`
	LoggedIn    bool   `json:"logged_in"`
}
