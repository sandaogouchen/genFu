package generate

import (
	"genFu/internal/message"
	"genFu/internal/tool"
)

type GenerateRequest struct {
	SessionID string            `json:"session_id"`
	Messages  []message.Message `json:"messages"`
	Tools     []tool.ToolSpec   `json:"tools,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

type GenerateResponse struct {
	Message   message.Message   `json:"message"`
	ToolCalls []tool.ToolCall   `json:"tool_calls,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

type GenerateEvent struct {
	Type       string           `json:"type"`
	Delta      string           `json:"delta,omitempty"`
	Message    *message.Message `json:"message,omitempty"`
	ToolCall   *tool.ToolCall   `json:"tool_call,omitempty"`
	ToolResult *tool.ToolResult `json:"tool_result,omitempty"`
	Done       bool             `json:"done,omitempty"`
}
