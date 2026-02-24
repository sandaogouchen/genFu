package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/tool"
)

type PromptAgent struct {
	name         string
	capabilities []string
	prompt       string
}

func NewPromptAgent(name string, capabilities []string, promptPath string) (*PromptAgent, error) {
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, err
	}
	return &PromptAgent{name: name, capabilities: capabilities, prompt: strings.TrimSpace(string(data))}, nil
}

func (a *PromptAgent) Name() string {
	return a.name
}

func (a *PromptAgent) Capabilities() []string {
	return a.capabilities
}

func (a *PromptAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	last := lastUserMessage(req.Messages)
	resp := generate.GenerateResponse{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: buildPromptResponse(a.prompt, last),
		},
	}
	if call, ok := parseToolCall(last); ok {
		resp.ToolCalls = []tool.ToolCall{call}
	}
	return resp, nil
}

func buildPromptResponse(prompt string, user string) string {
	if user == "" {
		return prompt
	}
	return prompt + "\n\n用户输入:\n" + user
}

func parseToolCall(text string) (tool.ToolCall, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return tool.ToolCall{}, false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "call:") || strings.HasPrefix(lower, "tool:") {
		idx := strings.Index(trimmed, ":")
		if idx < 0 || idx+1 >= len(trimmed) {
			return tool.ToolCall{}, false
		}
		rest := strings.TrimSpace(trimmed[idx+1:])
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) == 0 {
			return tool.ToolCall{}, false
		}
		toolName := strings.TrimSpace(parts[0])
		payload := ""
		if len(parts) == 2 {
			payload = strings.TrimSpace(parts[1])
		}
		return buildToolCall(toolName, payload)
	}
	if strings.HasPrefix(lower, "investment:") {
		payload := strings.TrimSpace(trimmed[len("investment:"):])
		return buildToolCall("investment", payload)
	}
	if strings.HasPrefix(lower, "echo:") {
		payload := strings.TrimSpace(trimmed[len("echo:"):])
		return buildToolCall("echo", payload)
	}
	return tool.ToolCall{}, false
}

func buildToolCall(name string, payload string) (tool.ToolCall, bool) {
	if name == "" {
		return tool.ToolCall{}, false
	}
	args := map[string]interface{}{}
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &args); err != nil {
			return tool.ToolCall{}, false
		}
	}
	if name == "investment" {
		if _, ok := args["action"]; !ok {
			args["action"] = "help"
		}
	}
	return tool.ToolCall{Name: name, Arguments: args}, true
}
