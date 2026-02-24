package kline

import (
	"github.com/cloudwego/eino/components/model"

	"genFu/internal/agent"
	"genFu/internal/tool"
)

func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile("kline", []string{"kline", "analysis"}, "internal/agent/prompt/kline.md", model, registry)
}
