package investment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"genFu/internal/db"
	"genFu/internal/testutil"
)

func TestRepositoryBasics(t *testing.T) {
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
	repo := NewRepository(conn)
	user, err := repo.CreateUser(ctx, "u")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := repo.CreateAccount(ctx, user.ID, "a", "CNY")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	instrument, err := repo.UpsertInstrument(ctx, "AAA", "AAA", "stock")
	if err != nil {
		t.Fatalf("upsert instrument: %v", err)
	}
	pos, err := repo.SetPosition(ctx, account.ID, instrument.ID, 1, 2, nil)
	if err != nil {
		t.Fatalf("set position: %v", err)
	}
	if pos.Instrument.Symbol != "AAA" {
		t.Fatalf("unexpected instrument")
	}
	fetched, err := repo.GetPosition(ctx, account.ID, instrument.ID)
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if fetched.Instrument.Symbol != "AAA" {
		t.Fatalf("unexpected fetched instrument")
	}
	if err := repo.DeletePosition(ctx, account.ID, instrument.ID); err != nil {
		t.Fatalf("delete position: %v", err)
	}
	positions, err := repo.ListPositions(ctx, account.ID)
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("expected empty positions")
	}
}
