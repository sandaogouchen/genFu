package regime

import (
	"genFu/internal/agent"
	"genFu/internal/tool"

	"github.com/cloudwego/eino/components/model"
)

// New 创建市场状态识别Agent
func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile(
		"regime_agent",
		[]string{"market_regime_detection", "market_state_classification"},
		"internal/agent/regime/prompt.md",
		model,
		registry,
	)
}
