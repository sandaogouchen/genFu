package agent

import (
	"context"

	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/tool"
)

type FuncAgent struct {
	name         string
	capabilities []string
	handler      func(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error)
}

func NewFuncAgent(name string, capabilities []string, handler func(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error)) *FuncAgent {
	return &FuncAgent{name: name, capabilities: capabilities, handler: handler}
}

func (a *FuncAgent) Name() string {
	return a.name
}

func (a *FuncAgent) Capabilities() []string {
	return a.capabilities
}

func (a *FuncAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	if a.handler == nil {
		return generate.GenerateResponse{}, nil
	}
	return a.handler(ctx, req)
}

func DefaultGenerate(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	last := lastUserMessage(req.Messages)
	return generate.GenerateResponse{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: last,
		},
	}, nil
}

func ToolGenerate(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	last := lastUserMessage(req.Messages)
	resp := generate.GenerateResponse{
		Message: message.Message{
			Role: message.RoleAssistant,
		},
	}
	if call, ok := parseToolCall(last); ok {
		resp.ToolCalls = []tool.ToolCall{call}
	}
	return resp, nil
}
