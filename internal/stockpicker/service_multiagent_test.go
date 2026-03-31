package stockpicker

import (
	"encoding/json"
	"testing"
)

func TestNormalizeRiskProfile(t *testing.T) {
	cases := map[string]string{
		"":             "balanced",
		"balanced":     "balanced",
		"conservative": "conservative",
		"aggressive":   "aggressive",
		"UNKNOWN":      "balanced",
	}
	for in, want := range cases {
		if got := normalizeRiskProfile(in); got != want {
			t.Fatalf("normalizeRiskProfile(%q)=%q want=%q", in, got, want)
		}
	}
}

func TestParseRegimeOutput(t *testing.T) {
	raw := "```json\n{\"market_regime\":\"trend_up\",\"regime_confidence\":0.88,\"regime_reasoning\":\"breadth strong\",\"regime_signals\":[\"up_count high\",\"limit_up strong\"]}\n```"
	out, err := parseRegimeOutput(raw)
	if err != nil {
		t.Fatalf("parseRegimeOutput failed: %v", err)
	}
	if out.MarketRegime != "trend_up" {
		t.Fatalf("unexpected regime: %s", out.MarketRegime)
	}
	if out.RegimeConfidence != 0.88 {
		t.Fatalf("unexpected confidence: %v", out.RegimeConfidence)
	}
}

func TestApplyPortfolioFit_HardFilterAndSoftSort(t *testing.T) {
	svc := &Service{}
	stocks := []StockPick{
		{Symbol: "A", Confidence: 0.8, Allocation: Allocation{SuggestedWeight: 0.12}},
		{Symbol: "B", Confidence: 0.7, Allocation: Allocation{SuggestedWeight: 0.18}},
		{Symbol: "C", Confidence: 0.6, Allocation: Allocation{SuggestedWeight: 0.10}},
	}
	fit := &PortfolioFitOutput{Stocks: []PortfolioFitRecord{
		{Symbol: "A", FitScore: 0.5, RiskBudgetWeight: 0.10, FitReasons: []string{"ok"}},
		{Symbol: "B", FitScore: 0.9, RiskBudgetWeight: 0.25, FitReasons: []string{"too high"}}, // 超过balanced cap, 应剔除
		{Symbol: "C", FitScore: 0.8, RiskBudgetWeight: 0.12, FitReasons: []string{"better"}},
	}}

	out := svc.applyPortfolioFit(stocks, fit, "balanced")
	if len(out) != 2 {
		t.Fatalf("expected 2 stocks after hard filter, got %d", len(out))
	}
	if out[0].Symbol != "C" || out[1].Symbol != "A" {
		t.Fatalf("unexpected order: %s, %s", out[0].Symbol, out[1].Symbol)
	}
	if out[0].RiskBudgetWeight > 0.2 {
		t.Fatalf("risk_budget_weight should respect cap")
	}
}

func TestGuideV2ProjectionRoundTrip(t *testing.T) {
	v1 := `{"asset_type":"stock","symbol":"600519","buy_rules":[{"rule_id":"B1"}],"sell_rules":[{"rule_id":"S1"}],"risk_controls":{"stop_loss_price":1}}`
	v2 := convertLegacyGuideToV2(v1)
	if !json.Valid([]byte(v2)) {
		t.Fatalf("v2 should be valid json: %s", v2)
	}
	v1Projected := projectGuideV2ToV1(v2, "600519")
	if !json.Valid([]byte(v1Projected)) {
		t.Fatalf("projected v1 should be valid json: %s", v1Projected)
	}
}

func TestApplyCompiledTradeGuides(t *testing.T) {
	svc := &Service{}
	out := &AgentOutput{Stocks: []StockPick{{
		Symbol:            "600519",
		TradeGuideText:    "fallback",
		TradeGuideJSON:    `{"asset_type":"stock","symbol":"600519","buy_rules":[],"sell_rules":[],"risk_controls":{}}`,
		TradeGuideJSONV2:  `{"schema_version":"v2"}`,
		TradeGuideVersion: "v1",
	}}}

	compiled := &TradeGuideCompilerOutput{Stocks: []TradeGuideCompilerRecord{{
		Symbol:            "600519",
		TradeGuideText:    "compiled",
		TradeGuideJSONV2:  `{"schema_version":"v2","asset_type":"stock","symbol":"600519","entries":[],"exits":[],"risk_controls":{}}`,
		TradeGuideJSON:    `{"asset_type":"stock","symbol":"600519","buy_rules":[],"sell_rules":[],"risk_controls":{}}`,
		TradeGuideVersion: "v2",
	}}}

	svc.applyCompiledTradeGuides(out, compiled)
	if out.Stocks[0].TradeGuideVersion != "v2" {
		t.Fatalf("expected version v2, got %s", out.Stocks[0].TradeGuideVersion)
	}
	if out.Stocks[0].TradeGuideText != "compiled" {
		t.Fatalf("expected compiled text, got %s", out.Stocks[0].TradeGuideText)
	}
	if !json.Valid([]byte(out.Stocks[0].TradeGuideJSONV2)) || !json.Valid([]byte(out.Stocks[0].TradeGuideJSON)) {
		t.Fatalf("compiled guide json should be valid")
	}
}
