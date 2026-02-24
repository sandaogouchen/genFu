package summary

import (
	"github.com/cloudwego/eino/components/model"

	"genFu/internal/agent"
	"genFu/internal/tool"
)

func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile("summary", []string{"summary", "analysis"}, "internal/agent/prompt/summary.md", model, registry)
}
