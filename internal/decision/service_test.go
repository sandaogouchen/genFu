package decision

import (
	"context"
	"encoding/json"
	"testing"

	"genFu/internal/agent"
	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/testutil"
	"genFu/internal/tool"
	"genFu/internal/trade_signal"
)

type fakeAgent struct{}

func (f fakeAgent) Name() string           { return "fake" }
func (f fakeAgent) Capabilities() []string { return nil }
func (f fakeAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	_ = req
	raw := `{"decision_id":"d1","decisions":[{"account_id":1,"symbol":"AAPL","action":"buy","quantity":1,"price":10}]}`
	toolResults, _ := json.Marshal([]tool.ToolResult{{Name: "echo", Output: "ok"}})
	return generate.GenerateResponse{
		Message: message.Message{Role: message.RoleAssistant, Content: raw},
		Meta:    map[string]string{"tool_results": string(toolResults)},
	}, nil
}

type fakeTool struct{}

func (f fakeTool) Spec() tool.ToolSpec {
	return tool.ToolSpec{Name: "echo"}
}
func (f fakeTool) Execute(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	_ = ctx
	return tool.ToolResult{Name: "echo", Output: args["text"]}, nil
}

type fakeEngine struct{}

func (f fakeEngine) Execute(ctx context.Context, signals []trade_signal.TradeSignal) ([]trade_signal.ExecutionResult, error) {
	_ = ctx
	results := make([]trade_signal.ExecutionResult, 0, len(signals))
	for _, s := range signals {
		results = append(results, trade_signal.ExecutionResult{Signal: s, Status: "executed"})
	}
	return results, nil
}

func TestDecide(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	reg := tool.NewRegistry()
	reg.Register(fakeTool{})
	svc := NewService(fakeAgent{}, reg, fakeEngine{}, nil, nil, nil)
	resp, err := svc.Decide(context.Background(), DecisionRequest{AccountID: cfg.News.AccountID})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if resp.Decision.DecisionID != "d1" {
		t.Fatalf("unexpected decision id")
	}
	if len(resp.Signals) != 1 || resp.Signals[0].Symbol != "AAPL" {
		t.Fatalf("unexpected signals")
	}
	if len(resp.ToolResults) != 1 {
		t.Fatalf("expected tool results")
	}
	if len(resp.Executions) != 1 || resp.Executions[0].Status != "executed" {
		t.Fatalf("unexpected executions")
	}
	_ = agent.Agent(fakeAgent{})
}
