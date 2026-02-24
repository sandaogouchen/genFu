package stockpicker

import (
	"context"
	"testing"
)

func TestStockStrategyRouterToolFindTool(t *testing.T) {
	tool := NewStockStrategyRouterTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":     "find_tool",
		"up_count":   3200,
		"down_count": 900,
	})
	if err != nil {
		t.Fatalf("route failed: %v", err)
	}

	output, ok := result.Output.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}

	if output["strategy_tool"] != toolStrategyMomentumStrong {
		t.Fatalf("unexpected strategy_tool: %v", output["strategy_tool"])
	}
	if output["strategy_name"] != strategyMomentumStrong {
		t.Fatalf("unexpected strategy_name: %v", output["strategy_name"])
	}
}

func TestStockStrategyToolBuildConditions(t *testing.T) {
	tool := NewStockStrategyTechnicalBreakoutTool()
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"up_count":   2500,
		"down_count": 1800,
		"limit":      80,
	})
	if err != nil {
		t.Fatalf("strategy tool failed: %v", err)
	}

	output, ok := result.Output.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	conditions, ok := output["screening_conditions"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing screening_conditions")
	}

	if conditions["strategy_type"] != strategyTechnicalBreak {
		t.Fatalf("unexpected strategy_type: %v", conditions["strategy_type"])
	}
	if conditions["limit"] != 80 {
		t.Fatalf("unexpected limit: %v", conditions["limit"])
	}
	if conditions["macd_golden"] != true {
		t.Fatalf("expected macd_golden=true, got %v", conditions["macd_golden"])
	}
}
