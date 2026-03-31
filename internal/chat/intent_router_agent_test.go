package chat

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fakeIntentModel struct {
	content string
	err     error
}

func (m fakeIntentModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	_ = tools
	return m, nil
}

func (m fakeIntentModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	_ = ctx
	_ = input
	_ = opts
	if m.err != nil {
		return nil, m.err
	}
	return schema.AssistantMessage(m.content, nil), nil
}

func (m fakeIntentModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	_ = ctx
	_ = input
	_ = opts
	reader, writer := schema.Pipe[*schema.Message](1)
	writer.Close()
	return reader, nil
}

func TestIntentRouterExplicitPrefix(t *testing.T) {
	router := NewIntentRouterAgent(fakeIntentModel{})
	got, err := router.Route(context.Background(), RouteInput{
		LastUserMessage: "call:investment {\"action\":\"list_positions\"}",
	})
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if got.Intent != IntentPortfolio {
		t.Fatalf("unexpected intent: %s", got.Intent)
	}
	if got.Workflow != WorkflowChatPortfolio {
		t.Fatalf("unexpected workflow: %s", got.Workflow)
	}
	if got.Fallback {
		t.Fatalf("unexpected fallback")
	}
}

func TestIntentRouterLLMResult(t *testing.T) {
	router := NewIntentRouterAgent(fakeIntentModel{
		content: `{"intent":"decision","confidence":0.92,"reason":"need_trade_plan","slots":{"account_id":3,"report_ids":[1,2,0]}}`,
	})
	got, err := router.Route(context.Background(), RouteInput{LastUserMessage: "给我交易建议"})
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if got.Intent != IntentDecision {
		t.Fatalf("unexpected intent: %s", got.Intent)
	}
	if got.Workflow != WorkflowDomainDecision {
		t.Fatalf("unexpected workflow: %s", got.Workflow)
	}
	if got.Slots.AccountID != 3 {
		t.Fatalf("unexpected account id: %d", got.Slots.AccountID)
	}
	if len(got.Slots.ReportIDs) != 2 {
		t.Fatalf("unexpected report ids: %#v", got.Slots.ReportIDs)
	}
}

func TestIntentRouterLowConfidenceFallback(t *testing.T) {
	router := NewIntentRouterAgent(fakeIntentModel{
		content: `{"intent":"stockpicker","confidence":0.3,"reason":"uncertain","slots":{"top_n":10}}`,
	})
	got, err := router.Route(context.Background(), RouteInput{LastUserMessage: "帮我挑股票"})
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if got.Intent != IntentGeneralChat {
		t.Fatalf("unexpected intent: %s", got.Intent)
	}
	if !got.Fallback {
		t.Fatalf("expected fallback")
	}
}

func TestIntentRouterInvalidJSONFallback(t *testing.T) {
	router := NewIntentRouterAgent(fakeIntentModel{
		content: "not-json",
	})
	got, err := router.Route(context.Background(), RouteInput{LastUserMessage: "你好"})
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if got.Intent != IntentGeneralChat {
		t.Fatalf("unexpected intent: %s", got.Intent)
	}
	if !got.Fallback {
		t.Fatalf("expected fallback")
	}
}
