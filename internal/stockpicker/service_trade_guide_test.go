package stockpicker

import (
	"encoding/json"
	"strings"
	"testing"
)

type parsedGuide struct {
	AssetType string  `json:"asset_type"`
	Symbol    string  `json:"symbol"`
	PriceRef  float64 `json:"price_ref"`
	BuyRules  []struct {
		RuleID       string  `json:"rule_id"`
		TriggerValue float64 `json:"trigger_value"`
	} `json:"buy_rules"`
	SellRules []struct {
		RuleID       string  `json:"rule_id"`
		TriggerValue float64 `json:"trigger_value"`
	} `json:"sell_rules"`
	RiskControls struct {
		StopLossPrice   float64 `json:"stop_loss_price"`
		TakeProfitPrice float64 `json:"take_profit_price"`
	} `json:"risk_controls"`
}

func TestAttachTradeGuides_GeneratesPerStockGuide(t *testing.T) {
	svc := &Service{}
	output := &AgentOutput{
		Stocks: []StockPick{
			{
				Symbol:       "600519",
				Name:         "贵州茅台",
				CurrentPrice: 1620.5,
				TechnicalReasons: TechnicalReason{
					KeyLevels: []string{"支撑位:1580元", "压力位:1650元"},
				},
				OperationGuide: &OperationGuide{
					StopLoss:   "跌破1500元止损",
					TakeProfit: "突破1750元可考虑减仓",
				},
			},
		},
	}
	screeningOutput := &AgentScreeningOutput{
		StrategyName: "technical_breakout",
		ScreeningConditions: ScreeningRequest{
			StrategyType: "technical_breakout",
			MACDGolden:   boolPtr(true),
			MA5AboveMA20: boolPtr(true),
			VolumeSpike:  boolPtr(true),
		},
	}
	screeningResult := &ScreeningResult{
		StrategyType: "technical_breakout",
	}

	svc.attachTradeGuides(output, screeningOutput, screeningResult)

	stock := output.Stocks[0]
	if stock.TradeGuideText == "" {
		t.Fatalf("trade_guide_text should not be empty")
	}
	if stock.TradeGuideVersion != "v1" {
		t.Fatalf("unexpected trade_guide_version: %s", stock.TradeGuideVersion)
	}
	if !json.Valid([]byte(stock.TradeGuideJSON)) {
		t.Fatalf("trade_guide_json should be strict JSON, got: %s", stock.TradeGuideJSON)
	}

	var guide parsedGuide
	if err := json.Unmarshal([]byte(stock.TradeGuideJSON), &guide); err != nil {
		t.Fatalf("unmarshal trade_guide_json failed: %v", err)
	}
	if guide.AssetType != "stock" {
		t.Fatalf("unexpected asset_type: %s", guide.AssetType)
	}
	if len(guide.BuyRules) == 0 || len(guide.SellRules) == 0 {
		t.Fatalf("buy/sell rules should not be empty")
	}
}

func TestBuildTradeGuideForStock_FallbackThresholds(t *testing.T) {
	svc := &Service{}
	stock := &StockPick{
		Symbol:       "000001",
		Name:         "平安银行",
		CurrentPrice: 100,
	}

	text, raw := svc.buildTradeGuideForStock(
		stock,
		&AgentScreeningOutput{
			StrategyName: "trend_following_core",
			ScreeningConditions: ScreeningRequest{
				StrategyType: "trend_following_core",
			},
		},
		nil,
	)

	if text == "" {
		t.Fatalf("trade guide text should not be empty")
	}
	var guide parsedGuide
	if err := json.Unmarshal([]byte(raw), &guide); err != nil {
		t.Fatalf("invalid trade guide json: %v", err)
	}

	if len(guide.BuyRules) == 0 || len(guide.SellRules) == 0 {
		t.Fatalf("fallback rules should not be empty")
	}

	// 默认阈值：买入 1.02x，卖出 0.97x，止损 0.95x，止盈 1.10x
	if guide.BuyRules[0].TriggerValue != 102 {
		t.Fatalf("unexpected fallback buy threshold: %v", guide.BuyRules[0].TriggerValue)
	}
	if guide.SellRules[0].TriggerValue != 97 {
		t.Fatalf("unexpected fallback sell threshold: %v", guide.SellRules[0].TriggerValue)
	}
	if guide.RiskControls.StopLossPrice != 95 {
		t.Fatalf("unexpected fallback stop_loss: %v", guide.RiskControls.StopLossPrice)
	}
	if guide.RiskControls.TakeProfitPrice != 110 {
		t.Fatalf("unexpected fallback take_profit: %v", guide.RiskControls.TakeProfitPrice)
	}
}

func TestStockPickResponseJSON_WithoutStrategyGuideField(t *testing.T) {
	resp := StockPickResponse{
		PickID:      "pick_test",
		NewsSummary: "ok",
		Stocks: []StockPick{
			{
				Symbol:            "600519",
				Name:              "贵州茅台",
				TradeGuideText:    "text",
				TradeGuideJSON:    "{}",
				TradeGuideVersion: "v1",
			},
		},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	out := string(raw)
	if strings.Contains(out, "strategy_guide") {
		t.Fatalf("response should not contain strategy_guide: %s", out)
	}
	if !strings.Contains(out, "trade_guide_text") || !strings.Contains(out, "trade_guide_json") {
		t.Fatalf("response should contain per-stock trade guide fields: %s", out)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
