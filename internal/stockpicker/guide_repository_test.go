package stockpicker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"genFu/internal/db"
	"genFu/internal/testutil"
)

func setupGuideRepositoryTestDB(t *testing.T) *db.DB {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "guide-repository.db")
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
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := db.ApplyMigrations(context.Background(), conn); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return conn
}

func TestGuideRepository_SaveAndListGuidesBySymbol(t *testing.T) {
	conn := setupGuideRepositoryTestDB(t)
	repo := NewGuideRepository(conn)
	ctx := context.Background()

	first := &OperationGuide{
		Symbol:            "600519",
		PickID:            "pick-a",
		BuyConditions:     []Condition{{Type: "text", Description: "买入规则A"}},
		SellConditions:    []Condition{{Type: "text", Description: "卖出规则A"}},
		StopLoss:          "1500",
		TakeProfit:        "1750",
		RiskMonitors:      []string{"监控A"},
		TradeGuideText:    "A",
		TradeGuideJSON:    `{"asset_type":"stock","symbol":"600519"}`,
		TradeGuideVersion: "v1",
	}
	if err := repo.SaveGuide(ctx, first); err != nil {
		t.Fatalf("save first guide: %v", err)
	}

	second := &OperationGuide{
		Symbol:            "600519",
		PickID:            "pick-b",
		BuyConditions:     []Condition{{Type: "text", Description: "买入规则B"}},
		SellConditions:    []Condition{{Type: "text", Description: "卖出规则B"}},
		StopLoss:          "1480",
		TakeProfit:        "1780",
		RiskMonitors:      []string{"监控B"},
		TradeGuideText:    "B",
		TradeGuideJSON:    `{"asset_type":"stock","symbol":"600519","version":2}`,
		TradeGuideVersion: "v1",
	}
	if err := repo.SaveGuide(ctx, second); err != nil {
		t.Fatalf("save second guide: %v", err)
	}

	guides, err := repo.ListGuidesBySymbol(ctx, "600519")
	if err != nil {
		t.Fatalf("list guides: %v", err)
	}
	if len(guides) != 2 {
		t.Fatalf("expected 2 guides, got %d", len(guides))
	}
	if guides[0].ID != second.ID {
		t.Fatalf("expected newest first, got id=%d want=%d", guides[0].ID, second.ID)
	}
	if guides[0].TradeGuideJSON != second.TradeGuideJSON {
		t.Fatalf("unexpected trade_guide_json: %s", guides[0].TradeGuideJSON)
	}
	if guides[1].TradeGuideText != first.TradeGuideText {
		t.Fatalf("unexpected trade_guide_text: %s", guides[1].TradeGuideText)
	}
}

func TestGuideRepository_ListGuidesByPickID(t *testing.T) {
	conn := setupGuideRepositoryTestDB(t)
	repo := NewGuideRepository(conn)
	ctx := context.Background()

	guideA := &OperationGuide{
		Symbol:            "600519",
		PickID:            "pick-xyz",
		BuyConditions:     []Condition{{Type: "text", Description: "买入A"}},
		SellConditions:    []Condition{{Type: "text", Description: "卖出A"}},
		TradeGuideText:    "A",
		TradeGuideJSON:    `{"asset_type":"stock","symbol":"600519"}`,
		TradeGuideJSONV2:  `{"schema_version":"v2","asset_type":"stock","symbol":"600519"}`,
		TradeGuideVersion: "v2",
	}
	guideB := &OperationGuide{
		Symbol:            "000001",
		PickID:            "pick-xyz",
		BuyConditions:     []Condition{{Type: "text", Description: "买入B"}},
		SellConditions:    []Condition{{Type: "text", Description: "卖出B"}},
		TradeGuideText:    "B",
		TradeGuideJSON:    `{"asset_type":"stock","symbol":"000001"}`,
		TradeGuideJSONV2:  `{"schema_version":"v2","asset_type":"stock","symbol":"000001"}`,
		TradeGuideVersion: "v2",
	}
	if err := repo.SaveGuide(ctx, guideA); err != nil {
		t.Fatalf("save guideA: %v", err)
	}
	if err := repo.SaveGuide(ctx, guideB); err != nil {
		t.Fatalf("save guideB: %v", err)
	}

	got, err := repo.ListGuidesByPickID(ctx, "pick-xyz")
	if err != nil {
		t.Fatalf("list by pick_id: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 guides, got %d", len(got))
	}
	if got[0].PickID != "pick-xyz" || got[1].PickID != "pick-xyz" {
		t.Fatalf("unexpected pick ids: %s %s", got[0].PickID, got[1].PickID)
	}
}
