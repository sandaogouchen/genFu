package post_trade_review

import (
	"github.com/cloudwego/eino/components/model"

	"genFu/internal/agent"
	"genFu/internal/tool"
)

func New(chatModel model.ToolCallingChatModel, registry *tool.Registry) (agent.Agent, error) {
	return agent.NewLLMAgentFromFile(
		"post_trade_review",
		[]string{"decision", "review"},
		"internal/agent/prompt/post_trade_review.md",
		chatModel,
		registry,
	)
}
