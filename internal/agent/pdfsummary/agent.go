package pdfsummary

import (
	"genFu/internal/agent"
	"genFu/internal/tool"

	"github.com/cloudwego/eino/components/model"
)

func New(model model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile(
		"pdfsummary",
		[]string{"pdf_analysis", "financial_analysis", "summarization"},
		"internal/agent/pdfsummary/prompt.md",
		model,
		registry,
	)
}
