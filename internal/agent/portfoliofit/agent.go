package portfoliofit

import (
	"genFu/internal/agent"
	"genFu/internal/tool"

	"github.com/cloudwego/eino/components/model"
)

// New 创建组合匹配Agent
func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile(
		"portfolio_fit_agent",
		[]string{"portfolio_correlation", "risk_budgeting", "portfolio_constraints"},
		"internal/agent/portfoliofit/prompt.md",
		model,
		registry,
	)
}
