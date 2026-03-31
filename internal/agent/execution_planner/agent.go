package execution_planner

import (
	"github.com/cloudwego/eino/components/model"

	"genFu/internal/agent"
	"genFu/internal/tool"
)

func New(chatModel model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile(
		"execution_planner",
		[]string{"decision", "execution_plan"},
		"internal/agent/prompt/execution_planner.md",
		chatModel,
		registry,
	)
}
