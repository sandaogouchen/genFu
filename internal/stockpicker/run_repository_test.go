package stockpicker

import (
	"context"
	"testing"
)

func TestRunRepository_SaveAndGetByPickID(t *testing.T) {
	conn := setupGuideRepositoryTestDB(t)
	repo := NewRunRepository(conn)
	ctx := context.Background()

	err := repo.SaveByPickID(ctx, "pick_100", StockPickRunSnapshot{
		Request:       map[string]interface{}{"account_id": 1, "risk_profile": "balanced"},
		MarketData:    map[string]interface{}{"up_count": 2000, "down_count": 1800},
		Regime:        map[string]interface{}{"market_regime": "trend_up", "regime_confidence": 0.66},
		Routing:       map[string]interface{}{"strategy_name": "trend_following_core"},
		CandidatePool: map[string]interface{}{"returned_count": 30},
		Analysis:      map[string]interface{}{"stocks": []interface{}{map[string]interface{}{"symbol": "600519"}}},
		PortfolioFit:  map[string]interface{}{"summary": "ok"},
		TradeGuides:   map[string]interface{}{"stocks": []interface{}{map[string]interface{}{"symbol": "600519"}}},
		Warnings:      []string{"warn_a"},
		Status:        "completed",
	})
	if err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	record, err := repo.GetByPickID(ctx, "pick_100")
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if record == nil {
		t.Fatalf("record should not be nil")
	}
	if record.PickID != "pick_100" {
		t.Fatalf("unexpected pick_id: %s", record.PickID)
	}
	if record.Status != "completed" {
		t.Fatalf("unexpected status: %s", record.Status)
	}

	summary := repo.BuildSummary(record)
	if summary == nil {
		t.Fatalf("summary should not be nil")
	}
	if summary.PickID != "pick_100" {
		t.Fatalf("unexpected summary pick_id: %s", summary.PickID)
	}
}

func TestRunRepository_SaveFailedStatus(t *testing.T) {
	conn := setupGuideRepositoryTestDB(t)
	repo := NewRunRepository(conn)
	ctx := context.Background()

	err := repo.SaveByPickID(ctx, "pick_failed", StockPickRunSnapshot{
		Status:       "failed",
		ErrorMessage: "parse_output_failed",
	})
	if err != nil {
		t.Fatalf("save failed snapshot: %v", err)
	}

	record, err := repo.GetByPickID(ctx, "pick_failed")
	if err != nil {
		t.Fatalf("get failed snapshot: %v", err)
	}
	if record == nil {
		t.Fatalf("record should not be nil")
	}
	if record.ErrorMessage != "parse_output_failed" {
		t.Fatalf("unexpected error_message: %s", record.ErrorMessage)
	}
}
