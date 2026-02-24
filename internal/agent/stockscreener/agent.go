package stockscreener

import (
	"genFu/internal/agent"
	"genFu/internal/tool"

	"github.com/cloudwego/eino/components/model"
)

// New 创建筛选Agent
func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile(
		"stock_screener",
		[]string{"stock_screening", "strategy_generation"},
		"internal/agent/stockscreener/prompt.md",
		model,
		registry,
	)
}
