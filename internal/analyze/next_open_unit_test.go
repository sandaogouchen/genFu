package analyze

import (
	"context"
	"testing"
	"time"
)

func TestNextOpenBuildDataMarketFailure(t *testing.T) {
	service := &NextOpenGuideService{
		location: time.UTC,
		newsLimit: 10,
	}
	data, err := service.buildData(context.Background())
	if err != nil {
		t.Fatalf("build data: %v", err)
	}
	if _, ok := data["critical_warning"]; !ok {
		t.Fatalf("expected critical_warning")
	}
	if _, ok := data["market_metrics"]; ok {
		t.Fatalf("unexpected market_metrics")
	}
	if _, ok := data["news"]; ok {
		t.Fatalf("unexpected news")
	}
}
