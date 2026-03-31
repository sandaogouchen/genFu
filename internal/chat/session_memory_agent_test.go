package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/message"
)

type fakeMemoryModel struct {
	content string
	err     error
}

func (m fakeMemoryModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	_ = tools
	return m, nil
}

func (m fakeMemoryModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	_ = ctx
	_ = input
	_ = opts
	if m.err != nil {
		return nil, m.err
	}
	return schema.AssistantMessage(m.content, nil), nil
}

func (m fakeMemoryModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	_ = ctx
	_ = input
	_ = opts
	reader, writer := schema.Pipe[*schema.Message](1)
	writer.Close()
	return reader, nil
}

func TestSessionMemoryAgentSummarize(t *testing.T) {
	agent := NewSessionMemoryAgent(fakeMemoryModel{
		content: `{"summary":"用户关注仓位风险，待确认止损位。"} `,
	})
	summary, err := agent.Summarize(context.Background(), "旧摘要", []message.Message{
		{Role: message.RoleUser, Content: "请给我仓位建议"},
		{Role: message.RoleAssistant, Content: "先控制仓位到三成"},
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary == "" {
		t.Fatalf("expected summary")
	}
}

func TestSessionMemoryAgentSummarizeError(t *testing.T) {
	agent := NewSessionMemoryAgent(fakeMemoryModel{err: errors.New("boom")})
	_, err := agent.Summarize(context.Background(), "旧摘要", []message.Message{
		{Role: message.RoleUser, Content: "hi"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}
