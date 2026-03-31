package stockpicker

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

// MockDataProvider 模拟数据提供者
type MockDataProvider struct {
	marketData  MarketData
	newsEvents  []NewsEvent
	holdings    []Position
	stockList   []tool.MarketItem
	marketError error
	newsError   error
	stockError  error
}

func (m *MockDataProvider) GetMarketData(ctx context.Context, days int) (MarketData, error) {
	return m.marketData, m.marketError
}

func (m *MockDataProvider) GetRecentNews(ctx context.Context, days int, limit int) ([]NewsEvent, error) {
	return m.newsEvents, m.newsError
}

func (m *MockDataProvider) GetHoldings(ctx context.Context, accountID int64) ([]Position, error) {
	return m.holdings, nil
}

func (m *MockDataProvider) GetStockList(ctx context.Context) ([]tool.MarketItem, error) {
	return m.stockList, m.stockError
}

func (m *MockDataProvider) GetFinancialData(ctx context.Context, symbol string) (map[string]interface{}, error) {
	return nil, nil
}

// MockAgent 模拟Agent
type MockAgent struct {
	response string
}

func (m *MockAgent) Name() string           { return "mock" }
func (m *MockAgent) Capabilities() []string { return []string{"test"} }

func (m *MockAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	last := ""
	if n := len(req.Messages); n > 0 {
		last = req.Messages[n-1].Content
	}
	if strings.Contains(last, "识别当前市场状态") {
		return generate.GenerateResponse{
			Message: message.Message{Role: message.RoleAssistant, Content: `{"market_regime":"trend_up","regime_confidence":0.72,"regime_reasoning":"breadth strong","regime_signals":["up_count high","limit_up stable"]}`},
		}, nil
	}
	if strings.Contains(last, "筛选策略") {
		return generate.GenerateResponse{
			Message: message.Message{Role: message.RoleAssistant, Content: `{"strategy_name":"technical_breakout","strategy_description":"技术突破","screening_conditions":{"strategy_type":"technical_breakout","limit":50},"market_context":"ok","risk_notes":"ok"}`},
		}, nil
	}
	if strings.Contains(last, "组合约束重排") {
		return generate.GenerateResponse{
			Message: message.Message{Role: message.RoleAssistant, Content: `{"summary":"ok","stocks":[{"symbol":"600519","fit_score":0.8,"risk_budget_weight":0.15,"fit_reasons":["liquidity"],"hard_reject":false,"reject_reason":""}]}`},
		}, nil
	}
	if strings.Contains(last, "编译交易规则") {
		return generate.GenerateResponse{
			Message: message.Message{Role: message.RoleAssistant, Content: `{"stocks":[{"symbol":"600519","trade_guide_text":"compiled","trade_guide_json_v2":"{\"schema_version\":\"v2\",\"asset_type\":\"stock\",\"symbol\":\"600519\",\"entries\":[],\"exits\":[],\"risk_controls\":{}}","trade_guide_json":"{\"asset_type\":\"stock\",\"symbol\":\"600519\",\"buy_rules\":[],\"sell_rules\":[],\"risk_controls\":{}}","trade_guide_version":"v2"}]}`},
		}, nil
	}
	return generate.GenerateResponse{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: m.response,
		},
	}, nil
}

type MockEastMoneyTool struct{}

func (m MockEastMoneyTool) Spec() tool.ToolSpec {
	return tool.ToolSpec{
		Name:        "eastmoney",
		Description: "mock eastmoney",
		Params:      map[string]string{"action": "string"},
		Required:    []string{"action"},
	}
}

func (m MockEastMoneyTool) Execute(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	action, _ := args["action"].(string)
	switch action {
	case "get_stock_list":
		return tool.ToolResult{
			Name: "eastmoney",
			Output: []tool.MarketItem{
				{Code: "600519", Name: "贵州茅台", Price: 1620, Amount: 1e9},
			},
		}, nil
	case "get_stock_quote":
		return tool.ToolResult{
			Name:   "eastmoney",
			Output: tool.StockQuote{Code: "000001", Name: "上证指数", Price: 3000, ChangeRate: -0.5},
		}, nil
	default:
		return tool.ToolResult{Name: "eastmoney", Error: "unsupported_action"}, nil
	}
}

func TestStockPickRequest(t *testing.T) {
	req := StockPickRequest{
		AccountID: 1,
		TopN:      5,
	}

	if req.AccountID != 1 {
		t.Errorf("expected AccountID 1, got %d", req.AccountID)
	}
	if req.TopN != 5 {
		t.Errorf("expected TopN 5, got %d", req.TopN)
	}
}

func TestStockPickResponseJSON(t *testing.T) {
	resp := StockPickResponse{
		PickID:      "test_123",
		GeneratedAt: time.Now(),
		Stocks: []StockPick{
			{
				Symbol:         "600519",
				Name:           "贵州茅台",
				Industry:       "白酒",
				CurrentPrice:   1620.50,
				Recommendation: "buy",
				Confidence:     0.85,
				RiskLevel:      "medium",
				TechnicalReasons: TechnicalReason{
					Trend:               "上升趋势",
					VolumeSignal:        "放量",
					TechnicalIndicators: []string{"MACD金叉"},
					KeyLevels:           []string{"支撑位1580"},
					RiskPoints:          []string{"估值偏高"},
				},
				Allocation: Allocation{
					SuggestedWeight:        0.15,
					IndustryDiversity:      0.8,
					RiskExposure:           0.3,
					LiquidityScore:         0.95,
					CorrelationWithHolding: 0.25,
				},
			},
		},
		MarketData: MarketData{
			IndexQuotes: []IndexQuote{
				{Code: "000001", Name: "上证指数", Price: 3000, Change: 10, ChangeRate: 0.33},
			},
			MarketSentiment: "neutral",
			UpCount:         2000,
			DownCount:       2500,
		},
		NewsSummary: "市场震荡",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Errorf("failed to marshal response: %v", err)
	}

	var parsed StockPickResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	if len(parsed.Stocks) != 1 {
		t.Errorf("expected 1 stock, got %d", len(parsed.Stocks))
	}
	if parsed.Stocks[0].Symbol != "600519" {
		t.Errorf("expected symbol 600519, got %s", parsed.Stocks[0].Symbol)
	}
}

func TestHandlerHTTP(t *testing.T) {
	// 创建模拟Agent响应
	agentResponse := `{
		"stocks": [
			{
				"symbol": "600519",
				"name": "贵州茅台",
				"industry": "白酒",
				"current_price": 1620.50,
				"recommendation": "buy",
				"confidence": 0.85,
				"technical_reasons": {
					"trend": "上升趋势完好",
					"volume_signal": "放量突破",
					"technical_indicators": ["MACD金叉"],
					"key_levels": ["支撑位1580元"],
					"risk_points": ["估值偏高"]
				},
				"risk_level": "medium"
			}
		],
		"market_view": "市场震荡上行",
		"risk_notes": "注意控制仓位"
	}`

	// 创建模拟数据提供者
	provider := &MockDataProvider{
		marketData: MarketData{
			IndexQuotes:     []IndexQuote{},
			MarketSentiment: "neutral",
		},
		newsEvents: []NewsEvent{},
		holdings:   []Position{},
		stockList: []tool.MarketItem{
			{Code: "600519", Name: "贵州茅台", Price: 1620, Amount: 1e9},
		},
	}

	// 创建模拟Agent
	mockAgent := &MockAgent{response: agentResponse}

	registry := tool.NewRegistry()
	registry.Register(NewStockScreenerTool(registry))
	registry.Register(MockEastMoneyTool{})

	// 创建服务
	svc := NewService(mockAgent, mockAgent, mockAgent, mockAgent, mockAgent, registry, provider, nil, nil)
	handler := NewHandler(svc, nil)

	// 创建请求
	reqBody := StockPickRequest{AccountID: 1, TopN: 3}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/stockpicker", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// 验证响应
	t.Logf("Response status: %d", rr.Code)
	t.Logf("Response body: %s", rr.Body.String())

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestAllocationService(t *testing.T) {
	svc := NewAllocationService()

	candidate := &StockPick{
		Symbol:     "600519",
		Industry:   "白酒",
		Confidence: 0.85,
		RiskLevel:  "medium",
	}

	holdings := []Position{
		{Symbol: "000858", Name: "五粮液", Industry: "白酒", Value: 100000},
	}

	stockList := []tool.MarketItem{
		{Code: "600519", Name: "贵州茅台", Amount: 1e9},
	}

	allocation := svc.CalculateAllocation(candidate, holdings, stockList)

	if allocation.SuggestedWeight <= 0 || allocation.SuggestedWeight > 0.2 {
		t.Errorf("suggested weight should be between 0.05 and 0.2, got %f", allocation.SuggestedWeight)
	}
	if allocation.LiquidityScore <= 0 {
		t.Errorf("liquidity score should be positive, got %f", allocation.LiquidityScore)
	}
}
