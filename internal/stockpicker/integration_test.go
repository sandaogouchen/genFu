// +build integration

package stockpicker

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// TestStockPickerAPIIntegration 测试真实的选股API
// 运行方式: go test -tags=integration -run TestStockPickerAPIIntegration
func TestStockPickerAPIIntegration(t *testing.T) {
	// 假设服务运行在 localhost:8080
	baseURL := "http://localhost:8080"

	reqBody := StockPickRequest{
		AccountID: 1,
		TopN:      3,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/stockpicker", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	t.Logf("Response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("API returned non-200 status: %d, body: %v", resp.StatusCode, errResp)
	}

	var result StockPickResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 验证响应结构
	if result.PickID == "" {
		t.Error("pick_id should not be empty")
	}
	if result.GeneratedAt.IsZero() {
		t.Error("generated_at should not be zero")
	}
	if len(result.Stocks) == 0 {
		t.Error("should return at least one stock")
	}
	if len(result.Stocks) > reqBody.TopN {
		t.Errorf("returned more stocks than requested: got %d, want <= %d", len(result.Stocks), reqBody.TopN)
	}

	// 验证每只股票的数据
	for i, stock := range result.Stocks {
		t.Logf("Stock %d: %s (%s) - %s", i+1, stock.Name, stock.Symbol, stock.Recommendation)

		if stock.Symbol == "" {
			t.Errorf("stock %d: symbol should not be empty", i)
		}
		if stock.Name == "" {
			t.Errorf("stock %d: name should not be empty", i)
		}
		if stock.CurrentPrice <= 0 {
			t.Errorf("stock %d: current_price should be positive", i)
		}
		if stock.Confidence < 0 || stock.Confidence > 1 {
			t.Errorf("stock %d: confidence should be between 0 and 1", i)
		}
		if stock.TechnicalReasons.Trend == "" {
			t.Errorf("stock %d: trend should not be empty", i)
		}
	}

	t.Logf("Market sentiment: %s", result.MarketData.MarketSentiment)
	t.Logf("News summary: %s", result.NewsSummary)
	if len(result.Warnings) > 0 {
		t.Logf("Warnings: %v", result.Warnings)
	}
}
