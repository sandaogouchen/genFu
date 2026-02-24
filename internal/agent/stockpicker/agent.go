package stockpicker

import (
	"genFu/internal/agent"
	"genFu/internal/tool"

	"github.com/cloudwego/eino/components/model"
)

// New 创建选股Agent
func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile(
		"stockpicker",
		[]string{"stock_picking", "technical_analysis", "asset_allocation"},
		"internal/agent/stockpicker/prompt.md",
		model,
		registry,
	)
}
