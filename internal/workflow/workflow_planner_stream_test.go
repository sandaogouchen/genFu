package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/tool"
)

type scriptedAgent struct {
	name    string
	content string
	chunks  []string
}

func (a scriptedAgent) Name() string {
	return a.name
}

func (a scriptedAgent) Capabilities() []string {
	return nil
}

func (a scriptedAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	_ = req
	content := a.content
	if strings.TrimSpace(content) == "" {
		content = strings.Join(a.chunks, "")
	}
	return generate.GenerateResponse{
		Message: message.Message{Role: message.RoleAssistant, Content: content},
	}, nil
}

func (a scriptedAgent) HandleStream(ctx context.Context, req generate.GenerateRequest) (<-chan generate.GenerateEvent, error) {
	_ = ctx
	_ = req
	out := make(chan generate.GenerateEvent, len(a.chunks)+2)
	go func() {
		defer close(out)
		for _, chunk := range a.chunks {
			out <- generate.GenerateEvent{Type: "delta", Delta: chunk}
			time.Sleep(2 * time.Millisecond)
		}
		content := a.content
		if strings.TrimSpace(content) == "" {
			content = strings.Join(a.chunks, "")
		}
		out <- generate.GenerateEvent{
			Type: "message",
			Message: &message.Message{
				Role:    message.RoleAssistant,
				Content: content,
			},
		}
		out <- generate.GenerateEvent{Type: "done", Done: true}
	}()
	return out, nil
}

func TestWorkflowPlannerAgentDynamicPrune(t *testing.T) {
	planner := NewWorkflowPlannerAgent(nil, false)
	plan := planner.Plan(StockWorkflowInput{
		Symbol: "000001",
		Prompt: "只看多头，忽略新闻和持仓",
	})
	if plan.ShouldRun("holdings") {
		t.Fatalf("holdings should be pruned")
	}
	if plan.ShouldRun("holdings_market") {
		t.Fatalf("holdings_market should be pruned")
	}
	if plan.ShouldRun("news_fetch") || plan.ShouldRun("news_summary") {
		t.Fatalf("news nodes should be pruned")
	}
	if !plan.ShouldRun("target_market") {
		t.Fatalf("target_market should still run")
	}
	if !plan.ShouldRun("bull") {
		t.Fatalf("bull should still run")
	}
	if plan.ShouldRun("bear") || plan.ShouldRun("debate") {
		t.Fatalf("bear/debate should be pruned in bull-only mode")
	}
	if !plan.ShouldRun("summary") {
		t.Fatalf("summary should still run")
	}
}

func TestStockSSEHandlerStreamsNodeEvents(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(fakeTool{
		spec: tool.ToolSpec{Name: "eastmoney"},
		exec: func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
			_ = ctx
			if args["action"] != "get_stock_quote" {
				return tool.ToolResult{Name: "eastmoney", Error: "unsupported_action"}, nil
			}
			return tool.ToolResult{
				Name: "eastmoney",
				Output: tool.StockQuote{
					Code:       "000001",
					Name:       "平安银行",
					Price:      11.11,
					Change:     0.01,
					ChangeRate: 0.09,
				},
			}, nil
		},
	})

	wf := &StockWorkflow{
		registry:             reg,
		defaultNewsRoutes:    nil,
		bullAgent:            scriptedAgent{name: "bull", chunks: []string{"看多", "理由"}},
		bearAgent:            scriptedAgent{name: "bear", content: "看空理由"},
		debateAgent:          scriptedAgent{name: "debate", content: "辩论结论"},
		summaryAgent:         scriptedAgent{name: "summary", content: "最终建议"},
		workflowPlannerAgent: NewWorkflowPlannerAgent(nil, false),
	}

	handler := NewStockSSEHandler(wf)
	reqBody, err := json.Marshal(StockWorkflowInput{
		Symbol: "000001",
		Name:   "平安银行",
		Prompt: "正常流程",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/sse/workflow/stock", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "event: node_start") {
		t.Fatalf("expected node_start events, got=%s", body)
	}
	if !strings.Contains(body, "event: node_delta") {
		t.Fatalf("expected node_delta events, got=%s", body)
	}
	if !strings.Contains(body, "event: node_complete") {
		t.Fatalf("expected node_complete events, got=%s", body)
	}
	if !strings.Contains(body, "event: bull") {
		t.Fatalf("expected bull payload event")
	}
	if !strings.Contains(body, "event: complete") {
		t.Fatalf("expected complete event")
	}

	startIdx := strings.Index(body, "event: node_start\ndata: {\"node\":\"bull\"}")
	deltaIdx := strings.Index(body, "event: node_delta")
	completeIdx := strings.Index(body, "event: node_complete\ndata: {\"node\":\"bull\"}")
	bullIdx := strings.Index(body, "event: bull")
	if !(startIdx >= 0 && deltaIdx > startIdx && completeIdx > deltaIdx && bullIdx > completeIdx) {
		t.Fatalf("unexpected node event order")
	}
}
