//go:build integration
// +build integration

package tool

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestEastMoneyGetStockListIntegration 用于本地排查get_stock_list接口可用性。
// 运行:
//
//	go test ./internal/tool -tags=integration -run TestEastMoneyGetStockListIntegration -v
//
// Cookie配置:
//
//	服务运行时请在 config.yaml 的 eastmoney.cookie 配置浏览器Cookie串。
func TestEastMoneyGetStockListIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	tool := NewEastMoneyTool()
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":    "get_stock_list",
		"page":      1,
		"page_size": 200,
	})
	if err != nil {
		t.Fatalf("get_stock_list failed: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("tool error: %s", result.Error)
	}

	items, ok := result.Output.([]MarketItem)
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	if len(items) == 0 {
		t.Fatalf("empty stock list")
	}

	sampleCount := 5
	if len(items) < sampleCount {
		sampleCount = len(items)
	}
	payload := map[string]interface{}{
		"count":  len(items),
		"sample": items[:sampleCount],
	}
	raw, _ := json.MarshalIndent(payload, "", "  ")
	t.Logf("eastmoney_stock_list=%s", string(raw))
}
