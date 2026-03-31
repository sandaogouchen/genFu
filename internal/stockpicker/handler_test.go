package stockpicker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListOperationGuidesBySymbol_WithPickID(t *testing.T) {
	conn := setupGuideRepositoryTestDB(t)
	guideRepo := NewGuideRepository(conn)
	runRepo := NewRunRepository(conn)

	if err := guideRepo.SaveGuide(testCtx(), &OperationGuide{
		Symbol:            "600519",
		PickID:            "pick-handler-1",
		BuyConditions:     []Condition{{Type: "text", Description: "买入"}},
		SellConditions:    []Condition{{Type: "text", Description: "卖出"}},
		TradeGuideText:    "text",
		TradeGuideJSON:    `{"asset_type":"stock","symbol":"600519"}`,
		TradeGuideJSONV2:  `{"schema_version":"v2","asset_type":"stock","symbol":"600519"}`,
		TradeGuideVersion: "v2",
	}); err != nil {
		t.Fatalf("save guide: %v", err)
	}
	if err := runRepo.SaveByPickID(testCtx(), "pick-handler-1", StockPickRunSnapshot{
		Regime:   map[string]interface{}{"market_regime": "trend_up"},
		Warnings: []string{"warn"},
		Status:   "completed",
	}); err != nil {
		t.Fatalf("save run snapshot: %v", err)
	}

	h := NewHandler(&Service{runRepo: runRepo}, guideRepo)
	req := httptest.NewRequest(http.MethodGet, "/api/operation-guides?pick_id=pick-handler-1", nil)
	rr := httptest.NewRecorder()
	h.ListOperationGuidesBySymbol(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var out map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out["pick_id"] != "pick-handler-1" {
		t.Fatalf("unexpected pick_id: %v", out["pick_id"])
	}
	if _, ok := out["snapshot_summary"]; !ok {
		t.Fatalf("snapshot_summary should exist")
	}
}

func testCtx() context.Context {
	return context.Background()
}
