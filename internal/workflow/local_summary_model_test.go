//go:build live

package workflow

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/rsshub"
)

type localSummaryModel struct{}

func (m localSummaryModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m localSummaryModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	_ = ctx
	_ = opts
	var payload []rsshub.Item
	if len(input) > 0 {
		_ = json.Unmarshal([]byte(input[len(input)-1].Content), &payload)
	}
	summary := "no_news"
	if len(payload) > 0 && payload[0].Title != "" {
		summary = payload[0].Title
	}
	return schema.AssistantMessage(`{"summary":"`+summary+`","sentiment":"中性"}`, nil), nil
}

func (m localSummaryModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	_ = ctx
	_ = input
	_ = opts
	reader, writer := schema.Pipe[*schema.Message](1)
	go func() {
		writer.Send(schema.AssistantMessage("ok", nil), nil)
		writer.Close()
	}()
	return reader, nil
}
