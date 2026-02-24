package bear

import (
	"github.com/cloudwego/eino/components/model"

	"genFu/internal/agent"
	"genFu/internal/tool"
)

func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile("bear", []string{"bear", "analysis"}, "internal/agent/prompt/bear.md", model, registry)
}
