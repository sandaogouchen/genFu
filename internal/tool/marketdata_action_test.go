package tool

import (
	"context"
	"testing"
)

func TestNormalizeMarketDataAction(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		args       map[string]interface{}
		wantAction string
		wantCode   string
	}{
		{
			name:       "fund kline alias",
			action:     "get_kline_data_fund",
			args:       map[string]interface{}{},
			wantAction: "get_fund_kline",
		},
		{
			name:       "fund kline daily alias",
			action:     "get_kline_data_fund_daily",
			args:       map[string]interface{}{},
			wantAction: "get_fund_kline",
		},
		{
			name:       "fund intraday alias",
			action:     "get_intraday_data_fund",
			args:       map[string]interface{}{},
			wantAction: "get_fund_intraday",
		},
		{
			name:       "extract code from action tail",
			action:     "get_kline_data_fund_daily_014597",
			args:       map[string]interface{}{},
			wantAction: "get_fund_kline",
			wantCode:   "014597",
		},
		{
			name:       "keep canonical action",
			action:     "get_fund_kline",
			args:       map[string]interface{}{},
			wantAction: "get_fund_kline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeMarketDataAction(tt.args, tt.action)
			if got != tt.wantAction {
				t.Fatalf("action mismatch: got=%s want=%s", got, tt.wantAction)
			}
			code, ok := tt.args["code"].(string)
			if tt.wantCode == "" {
				if ok && code != "" {
					t.Fatalf("unexpected code injected: %q", code)
				}
				return
			}
			if !ok {
				t.Fatalf("expected code injected, got args=%v", tt.args)
			}
			if code != tt.wantCode {
				t.Fatalf("code mismatch: got=%s want=%s", code, tt.wantCode)
			}
		})
	}
}

func TestMarketDataExecute_AliasUsesCanonicalRoute(t *testing.T) {
	tool := NewMarketDataTool(nil)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_kline_data_fund",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "missing_param_code" {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "missing_param_code" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
}
