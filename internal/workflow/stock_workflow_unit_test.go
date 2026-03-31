package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/rsshub"
	"genFu/internal/tool"
)

type fakeTool struct {
	spec tool.ToolSpec
	exec func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error)
}

type payloadCaptureAgent struct {
	t       *testing.T
	payload map[string]json.RawMessage
}

func (a *payloadCaptureAgent) Name() string {
	return "capture"
}

func (a *payloadCaptureAgent) Capabilities() []string {
	return nil
}

func (a *payloadCaptureAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	if len(req.Messages) == 0 {
		a.t.Fatalf("missing request messages")
	}
	last := req.Messages[len(req.Messages)-1]
	if err := json.Unmarshal([]byte(last.Content), &a.payload); err != nil {
		a.t.Fatalf("unmarshal payload: %v", err)
	}
	return generate.GenerateResponse{
		Message: message.Message{Role: message.RoleAssistant, Content: "ok"},
	}, nil
}

func TestRunAgentPayloadExcludesHoldingsForBullBear(t *testing.T) {
	ag := &payloadCaptureAgent{t: t}
	_, err := runAgent(context.Background(), ag, agentContext{
		Symbol:        "002970",
		Name:          "锐明技术",
		TargetMarket:  MarketMove{Symbol: "002970", Name: "锐明技术", Price: 50.2},
		NewsSummary:   "订单增长",
		NewsSentiment: "利好",
	}, nil, workflowNodeBull)
	if err != nil {
		t.Fatalf("runAgent err: %v", err)
	}

	if _, ok := ag.payload["HoldingsPositions"]; ok {
		t.Fatalf("bull/bear payload should not include HoldingsPositions")
	}
	if _, ok := ag.payload["HoldingsTotalValue"]; ok {
		t.Fatalf("bull/bear payload should not include HoldingsTotalValue")
	}
	if _, ok := ag.payload["HoldingsMarket"]; ok {
		t.Fatalf("bull/bear payload should not include HoldingsMarket")
	}
	if _, ok := ag.payload["TargetMarket"]; !ok {
		t.Fatalf("bull/bear payload should include TargetMarket")
	}
}

func TestRunSummaryAgentPayloadIncludesHoldings(t *testing.T) {
	ag := &payloadCaptureAgent{t: t}
	_, err := runSummaryAgent(context.Background(), ag, summaryInput{
		Symbol:             "002970",
		Name:               "锐明技术",
		Bull:               "看多观点",
		Bear:               "看空观点",
		Debate:             "辩论结果",
		HoldingsPositions:  []HoldingPosition{{Symbol: "007345", Name: "示例基金", Value: 100}},
		HoldingsTotalValue: 100,
		TargetMarket:       MarketMove{Symbol: "002970", Name: "锐明技术", Price: 50.2},
	}, nil, workflowNodeSummary)
	if err != nil {
		t.Fatalf("runSummaryAgent err: %v", err)
	}

	if _, ok := ag.payload["HoldingsPositions"]; !ok {
		t.Fatalf("summary payload should include HoldingsPositions")
	}
	if _, ok := ag.payload["HoldingsTotalValue"]; !ok {
		t.Fatalf("summary payload should include HoldingsTotalValue")
	}
}

func (f fakeTool) Spec() tool.ToolSpec {
	return f.spec
}

func (f fakeTool) Execute(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	if f.exec == nil {
		return tool.ToolResult{Name: f.spec.Name, Error: "exec_not_implemented"}, errors.New("exec_not_implemented")
	}
	return f.exec(ctx, args)
}

func TestResolveWorkflowInstrumentExactMatch(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(fakeTool{
		spec: tool.ToolSpec{Name: "investment"},
		exec: func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
			return tool.ToolResult{
				Name: "investment",
				Output: []map[string]interface{}{
					{"symbol": "007345", "name": "富国科技创新灵活配置混合", "asset_type": "fund"},
				},
			}, nil
		},
	})
	wf := &StockWorkflow{registry: reg}

	out, err := wf.resolveWorkflowInstrument(context.Background(), StockWorkflowInput{
		Symbol: "富国 科技创新灵活配置混合",
	})
	if err != nil {
		t.Fatalf("resolve err: %v", err)
	}
	if out.Symbol != "007345" {
		t.Fatalf("expected resolved symbol, got=%s", out.Symbol)
	}
	if out.AssetType != "fund" {
		t.Fatalf("expected fund asset type, got=%s", out.AssetType)
	}
	if out.Name != "富国科技创新灵活配置混合" {
		t.Fatalf("expected canonical name, got=%s", out.Name)
	}
}

func TestResolveWorkflowInstrumentNoExactMatch(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(fakeTool{
		spec: tool.ToolSpec{Name: "investment"},
		exec: func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
			return tool.ToolResult{
				Name: "investment",
				Output: []map[string]interface{}{
					{"symbol": "513180", "name": "恒生科技ETF", "asset_type": "fund"},
				},
			}, nil
		},
	})
	wf := &StockWorkflow{registry: reg}

	_, err := wf.resolveWorkflowInstrument(context.Background(), StockWorkflowInput{
		Symbol: "富国科技创新灵活配置混合",
	})
	if err == nil {
		t.Fatalf("expected no exact match error")
	}
	if !strings.Contains(err.Error(), "fund_search_not_exact_match") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchQuoteByAssetFallbackToFund(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(fakeTool{
		spec: tool.ToolSpec{Name: "eastmoney"},
		exec: func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
			if args["action"] == "get_stock_quote" {
				return tool.ToolResult{Name: "eastmoney", Error: "empty_response"}, errors.New("empty_response")
			}
			return tool.ToolResult{Name: "eastmoney", Error: "unsupported_action"}, errors.New("unsupported_action")
		},
	})
	reg.Register(fakeTool{
		spec: tool.ToolSpec{Name: "marketdata"},
		exec: func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
			if args["action"] == "get_fund_intraday" {
				return tool.ToolResult{
					Name: "marketdata",
					Output: []tool.IntradayPoint{
						{Time: "10:00", Price: 0},
						{Time: "10:01", Price: 1.026},
					},
				}, nil
			}
			return tool.ToolResult{Name: "marketdata", Error: "unsupported_action"}, errors.New("unsupported_action")
		},
	})

	move := fetchQuoteByAsset(context.Background(), reg, "007345", "富国科技创新灵活配置混合", "")
	if move.Error != "" {
		t.Fatalf("unexpected move error: %s", move.Error)
	}
	if move.Price != 1.026 {
		t.Fatalf("unexpected fund fallback price: %.3f", move.Price)
	}
}

func TestBuildHoldingsMarketUsesFundPathByAssetType(t *testing.T) {
	reg := tool.NewRegistry()
	fundCalls := 0
	stockCalls := 0

	reg.Register(fakeTool{
		spec: tool.ToolSpec{Name: "eastmoney"},
		exec: func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
			if args["action"] != "get_stock_quote" {
				return tool.ToolResult{Name: "eastmoney", Error: "unsupported_action"}, errors.New("unsupported_action")
			}
			stockCalls++
			return tool.ToolResult{
				Name: "eastmoney",
				Output: tool.StockQuote{
					Code:       "600519",
					Name:       "贵州茅台",
					Price:      1500.0,
					Change:     10.0,
					ChangeRate: 0.6,
				},
			}, nil
		},
	})
	reg.Register(fakeTool{
		spec: tool.ToolSpec{Name: "marketdata"},
		exec: func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
			if args["action"] != "get_fund_intraday" {
				return tool.ToolResult{Name: "marketdata", Error: "unsupported_action"}, errors.New("unsupported_action")
			}
			fundCalls++
			return tool.ToolResult{
				Name: "marketdata",
				Output: []tool.IntradayPoint{
					{Time: "14:59", Price: 0},
					{Time: "15:00", Price: 1.113},
				},
			}, nil
		},
	})

	out, err := buildHoldingsMarket(context.Background(), reg, holdingsMarketInput{
		Positions: []HoldingPosition{
			{Symbol: "007345", Name: "富国科技创新灵活配置混合", AssetType: "fund"},
			{Symbol: "600519", Name: "贵州茅台", AssetType: "stock"},
		},
	})
	if err != nil {
		t.Fatalf("buildHoldingsMarket err: %v", err)
	}
	if fundCalls != 1 {
		t.Fatalf("expected one fund call, got=%d", fundCalls)
	}
	if stockCalls != 1 {
		t.Fatalf("expected one stock call, got=%d", stockCalls)
	}
	if len(out.Quotes) != 2 {
		t.Fatalf("expected 2 quotes, got=%d", len(out.Quotes))
	}
	if out.Quotes[0].Price != 1.113 {
		t.Fatalf("unexpected fund quote price: %.3f", out.Quotes[0].Price)
	}
	if out.Quotes[1].Price != 1500.0 {
		t.Fatalf("unexpected stock quote price: %.3f", out.Quotes[1].Price)
	}
}

func TestFetchNewsUsesDefaultRoutesWhenRequestEmpty(t *testing.T) {
	reg := tool.NewRegistry()
	var usedRoute string
	reg.Register(fakeTool{
		spec: tool.ToolSpec{Name: "rsshub"},
		exec: func(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
			if args["action"] != "fetch_feed" {
				return tool.ToolResult{Name: "rsshub", Error: "unsupported_action"}, errors.New("unsupported_action")
			}
			usedRoute, _ = args["route"].(string)
			return tool.ToolResult{
				Name: "rsshub",
				Output: []rsshub.Item{
					{Title: "默认路由新闻", Link: "https://example.com/a"},
				},
			}, nil
		},
	})

	out, err := fetchNews(context.Background(), reg, []string{"/eastmoney/search/科技"}, newsInput{})
	if err != nil {
		t.Fatalf("fetchNews err: %v", err)
	}
	if usedRoute != "/eastmoney/search/科技" {
		t.Fatalf("unexpected route used: %s", usedRoute)
	}
	if len(out.Items) != 1 {
		t.Fatalf("expected one news item, got=%d", len(out.Items))
	}
}
