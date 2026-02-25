package stockpicker

import (
	"encoding/json"
	"testing"
)

func TestParseAgentOutput_NormalizeStrategyGuideTradeSignals(t *testing.T) {
	svc := &Service{}

	content := `{
		"stocks": [],
		"market_view": "震荡市",
		"risk_notes": "控制仓位",
		"strategy_guide": {
			"strategy_type": "technical_breakout",
			"strategy_name": "技术突破",
			"guide_text": "满足信号时分批交易",
			"trade_signals_json": {
				"strategy_type": "technical_breakout",
				"buy_signals": [{"indicator":"MACD","signal":"金叉","action":"buy"}],
				"sell_signals": [{"indicator":"MACD","signal":"死叉","action":"sell"}],
				"risk_controls": ["单票仓位<20%"]
			}
		}
	}`

	out, err := svc.parseAgentOutput(content)
	if err != nil {
		t.Fatalf("parseAgentOutput failed: %v", err)
	}
	if out.StrategyGuide == nil {
		t.Fatalf("strategy guide should not be nil")
	}
	if out.StrategyGuide.TradeSignalsJSON == "" {
		t.Fatalf("trade_signals_json should not be empty")
	}
	if !json.Valid([]byte(out.StrategyGuide.TradeSignalsJSON)) {
		t.Fatalf("trade_signals_json should be valid JSON, got: %s", out.StrategyGuide.TradeSignalsJSON)
	}
}

func TestEnsureStrategyGuide_FallbackWhenMissing(t *testing.T) {
	svc := &Service{}

	output := &AgentOutput{}
	screening := &AgentScreeningOutput{
		StrategyName: "technical_breakout",
		ScreeningConditions: ScreeningRequest{
			StrategyType: "technical_breakout",
			MACDGolden:   boolPtr(true),
			MA5AboveMA20: boolPtr(true),
		},
	}

	svc.ensureStrategyGuide(output, screening)

	if output.StrategyGuide == nil {
		t.Fatalf("strategy guide should not be nil")
	}
	if output.StrategyGuide.StrategyType != "technical_breakout" {
		t.Fatalf("unexpected strategy_type: %s", output.StrategyGuide.StrategyType)
	}
	if output.StrategyGuide.GuideText == "" {
		t.Fatalf("guide text should not be empty")
	}
	if !json.Valid([]byte(output.StrategyGuide.TradeSignalsJSON)) {
		t.Fatalf("fallback trade_signals_json should be valid JSON, got: %s", output.StrategyGuide.TradeSignalsJSON)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
