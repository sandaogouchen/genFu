package analyze

import (
	"testing"
	"time"

	"genFu/internal/testutil"
	"genFu/internal/tool"
)

func TestFupanDate(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.LLM.Endpoint == "" {
		t.Fatalf("missing config")
	}
	svc := &DailyReviewService{location: time.FixedZone("CST", 8*3600)}
	d := svc.fupanDate(time.Date(2026, 2, 16, 10, 0, 0, 0, svc.location))
	if d != "20260213" {
		t.Fatalf("unexpected date: %s", d)
	}
}

func TestComputeMarketMetrics(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.PG.DSN == "" {
		t.Fatalf("missing config")
	}
	svc := &DailyReviewService{}
	items := []tool.MarketItem{
		{Code: "1", Amount: 10, ChangeRate: 1, Amplitude: 2},
		{Code: "2", Amount: 20, ChangeRate: -1, Amplitude: 3},
	}
	metrics := svc.computeMarketMetrics(items)
	if metrics["up"].(int) != 1 || metrics["down"].(int) != 1 {
		t.Fatalf("unexpected counts")
	}
	if len(metrics["top_amount"].([]tool.MarketItem)) != 2 {
		t.Fatalf("unexpected top amount")
	}
}

func TestIsTradingDay(t *testing.T) {
	weekday := time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC)
	if !isTradingDay(weekday) {
		t.Fatalf("expected trading day")
	}
	weekend := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	if isTradingDay(weekend) {
		t.Fatalf("expected non-trading day")
	}
}
