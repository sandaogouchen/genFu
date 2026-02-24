package fundmanager

import (
	"github.com/cloudwego/eino/components/model"

	"genFu/internal/agent"
	"genFu/internal/tool"
)

func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile("fund_manager", []string{"fund_manager", "analysis"}, "internal/agent/prompt/fund_manager.md", model, registry)
}
