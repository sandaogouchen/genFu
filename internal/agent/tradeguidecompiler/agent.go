package tradeguidecompiler

import (
	"genFu/internal/agent"
	"genFu/internal/tool"

	"github.com/cloudwego/eino/components/model"
)

// New 创建交易指南编译Agent
func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile(
		"trade_guide_compiler_agent",
		[]string{"trade_rule_compilation", "rule_normalization", "json_schema_compilation"},
		"internal/agent/tradeguidecompiler/prompt.md",
		model,
		registry,
	)
}
