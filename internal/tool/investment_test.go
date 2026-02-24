package tool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"genFu/internal/db"
	"genFu/internal/investment"
	"genFu/internal/testutil"
)

func TestInvestmentToolListFundHoldings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
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
		t.Fatalf("open: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("wd: %v", err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	if err := db.ApplyMigrations(context.Background(), conn); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	ctx := context.Background()
	repo := investment.NewRepository(conn)
	svc := investment.NewService(repo)
	user, err := repo.CreateUser(ctx, "u")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := repo.CreateAccount(ctx, user.ID, "a", "CNY")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	fundInstrument, err := repo.UpsertInstrument(ctx, "FUND1", "fund", "fund")
	if err != nil {
		t.Fatalf("fund instrument: %v", err)
	}
	stockInstrument, err := repo.UpsertInstrument(ctx, "STK1", "stock", "stock")
	if err != nil {
		t.Fatalf("stock instrument: %v", err)
	}
	if _, err := repo.SetPosition(ctx, account.ID, fundInstrument.ID, 1, 1, nil); err != nil {
		t.Fatalf("set fund position: %v", err)
	}
	if _, err := repo.SetPosition(ctx, account.ID, stockInstrument.ID, 1, 1, nil); err != nil {
		t.Fatalf("set stock position: %v", err)
	}
	tool := NewInvestmentTool(svc)
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "list_fund_holdings",
		"account_id": account.ID,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	positions, ok := result.Output.([]investment.Position)
	if !ok {
		t.Fatalf("unexpected output type")
	}
	if len(positions) != 1 {
		t.Fatalf("unexpected positions length: %d", len(positions))
	}
	if positions[0].Instrument.AssetType != "fund" {
		t.Fatalf("unexpected asset type")
	}
}
