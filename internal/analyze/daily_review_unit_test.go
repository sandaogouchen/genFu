package analyze

import (
	"context"
	"testing"
	"time"
)

func TestDailyReviewBuildDataMarketFailure(t *testing.T) {
	service := &DailyReviewService{
		location: time.UTC,
	}
	now := time.Date(2026, 2, 12, 15, 30, 0, 0, time.UTC)
	ctx := context.WithValue(context.Background(), "skip_fupan", true)
	data, err := service.buildData(ctx, now)
	if err != nil {
		t.Fatalf("build data: %v", err)
	}
	if _, ok := data["critical_warning"]; !ok {
		t.Fatalf("expected critical_warning")
	}
	if _, ok := data["indexes"]; ok {
		t.Fatalf("unexpected indexes")
	}
	if _, ok := data["sector_heatmap"]; ok {
		t.Fatalf("unexpected sector_heatmap")
	}
	if _, ok := data["capital_flow"]; ok {
		t.Fatalf("unexpected capital_flow")
	}
	if _, ok := data["sentiment"]; ok {
		t.Fatalf("unexpected sentiment")
	}
}
