package news

import "testing"

func TestBuildExposureMapping_IncludesHoldingsAndWatchlist(t *testing.T) {
	portfolio := &PortfolioContext{
		Holdings: []Holding{
			{
				Name:        "台积电",
				Code:        "TSM",
				Weight:      0.25,
				Products:    []string{"先进制程"},
				Competitors: []string{"三星"},
				SupplyChain: []string{"ASML"},
			},
		},
		Watchlist: []WatchItem{
			{Name: "英伟达", Code: "NVDA"},
		},
		IndustryThemes: []string{"半导体"},
		MacroFactors:   []string{"美联储利率政策"},
	}

	event := &NewsEvent{Title: "先进制程受限"}
	mapping := ImpactMapping{
		EventSummary: "先进制程供给收紧",
		Items: []ImpactItem{
			{
				EntityName:  "先进制程",
				Direction:   DirectionBearish,
				ImpactScore: -0.8,
				ImpactLevel: ImpactLevelStrong,
				Confidence:  0.9,
			},
			{
				EntityName:  "英伟达",
				EntityCode:  "NVDA",
				Direction:   DirectionBearish,
				ImpactScore: -0.5,
				ImpactLevel: ImpactLevelModerate,
				Confidence:  0.8,
			},
		},
	}

	exposures := BuildExposureMapping(event, mapping, portfolio)
	if len(exposures) == 0 {
		t.Fatalf("expected non-empty exposures")
	}

	var hasHolding, hasWatchlist bool
	for _, e := range exposures {
		if e.Bucket == ExposureBucketHolding {
			hasHolding = true
		}
		if e.Bucket == ExposureBucketWatchlist {
			hasWatchlist = true
		}
	}

	if !hasHolding {
		t.Fatalf("expected holding exposure")
	}
	if !hasWatchlist {
		t.Fatalf("expected watchlist exposure")
	}
}

func TestApplyVerificationPenalty_DowngradesButDoesNotBlock(t *testing.T) {
	event := &NewsEvent{
		FunnelResult: &FunnelResult{
			L2Pass:     true,
			L2Priority: 5,
			L2AffectedAssets: []AssetImpact{
				{AssetName: "TSM", Confidence: 0.9, Direction: DirectionBearish, IsHolding: true},
			},
			ExposureMapping: []PortfolioExposure{
				{AssetName: "TSM", Bucket: ExposureBucketHolding, Confidence: 0.9, ExposureScore: 0.8},
			},
		},
	}

	verification := CausalVerification{Verdict: VerificationVerdictWeak, Score: 0.45, Reason: "证据不足"}
	applyVerificationPenalty(event, verification)

	if !event.FunnelResult.L2Pass {
		t.Fatalf("expected L2Pass to remain true")
	}
	if event.FunnelResult.L2Priority >= 5 {
		t.Fatalf("expected L2Priority downgraded, got=%d", event.FunnelResult.L2Priority)
	}
	if got := event.FunnelResult.L2AffectedAssets[0].Confidence; got >= 0.9 {
		t.Fatalf("expected affected asset confidence downgraded, got=%f", got)
	}
}

func TestBackfillLegacyMonitoringSignalsFromV2(t *testing.T) {
	event := &NewsEvent{FunnelResult: &FunnelResult{}}
	event.FunnelResult.MonitoringSignalsV2 = []StructuredMonitoringSignal{
		{
			Signal:    "TSM库存天数上升",
			Metric:    "inventory_days",
			Operator:  SignalOperatorGreaterThan,
			Threshold: "95",
			Window:    "7d",
			Assets:    []string{"TSM"},
			Reason:    "需求转弱",
		},
	}

	backfillLegacyMonitoringSignals(event)
	if event.FunnelResult.L3Analysis == nil {
		t.Fatalf("expected L3Analysis to be initialized")
	}
	if len(event.FunnelResult.L3Analysis.MonitoringSignals) == 0 {
		t.Fatalf("expected monitoring signals to be backfilled")
	}
}
