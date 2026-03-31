package decision

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"genFu/internal/db"
	"genFu/internal/testutil"
	"genFu/internal/trade_signal"
)

func setupDecisionRepository(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "decision-repo.db")
	cfg := testutil.LoadConfig(t)
	dbCfg := cfg.PG
	dbCfg.DSN = "file:" + path
	conn, err := db.Open(db.Config{
		DSN:             dbCfg.DSN,
		MaxOpenConns:    dbCfg.MaxOpenConns,
		MaxIdleConns:    dbCfg.MaxIdleConns,
		ConnMaxLifetime: dbCfg.ConnMaxLifetime,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("wd: %v", err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := db.ApplyMigrations(context.Background(), conn); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return NewRepository(conn)
}

func TestDecisionRepositoryLifecycle(t *testing.T) {
	repo := setupDecisionRepository(t)
	ctx := context.Background()
	runID, err := repo.CreateRun(
		ctx,
		1,
		"d-001",
		DecisionRequest{AccountID: 1, ReportIDs: []int64{11, 12}},
		trade_signal.DecisionOutput{
			DecisionID: "d-001",
			MarketView: "neutral",
		},
		DefaultRiskBudget(),
	)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if runID <= 0 {
		t.Fatalf("invalid run id: %d", runID)
	}

	err = repo.SaveOrders(ctx, runID, []GuardedOrder{
		{
			PlannedOrder: PlannedOrder{
				OrderID:        "d-001-1",
				AccountID:      1,
				Symbol:         "600519",
				Name:           "贵州茅台",
				AssetType:      "stock",
				Action:         "buy",
				Quantity:       1,
				Price:          1000,
				Notional:       1000,
				Confidence:     0.9,
				DecisionID:     "d-001",
				PlanningReason: "test",
			},
			GuardStatus:     "approved",
			ExecutionStatus: "executed",
			TradeID:         99,
		},
	})
	if err != nil {
		t.Fatalf("save orders: %v", err)
	}

	review := &PostTradeReview{
		Summary: "ok",
		Attributions: []ReviewAttribution{
			{OrderID: "d-001-1", Title: "执行正常", Detail: "无异常"},
		},
		LearningPoints: []string{"保持仓位纪律"},
	}
	if err := repo.SaveReview(ctx, runID, review); err != nil {
		t.Fatalf("save review: %v", err)
	}
	if err := repo.FinalizeRun(ctx, runID, "completed"); err != nil {
		t.Fatalf("finalize run: %v", err)
	}

	saved, err := repo.GetReviewByRunID(ctx, runID)
	if err != nil {
		t.Fatalf("get review: %v", err)
	}
	if saved == nil || saved.Summary != "ok" {
		t.Fatalf("unexpected review: %#v", saved)
	}
}
