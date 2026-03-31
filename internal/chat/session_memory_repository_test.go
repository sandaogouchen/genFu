package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"genFu/internal/db"
	"genFu/internal/testutil"
)

func TestSessionMemoryRepositoryUpsertGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat-memory.db")
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

	repo := NewSessionMemoryRepository(conn)
	ctx := context.Background()
	sessionID := "s1"
	if err := repo.Upsert(ctx, sessionID, "用户想跟踪持仓波动", string(IntentPortfolio)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := repo.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Summary != "用户想跟踪持仓波动" {
		t.Fatalf("unexpected summary: %s", got.Summary)
	}
	if got.LastIntent != string(IntentPortfolio) {
		t.Fatalf("unexpected last_intent: %s", got.LastIntent)
	}
	if err := repo.Upsert(ctx, sessionID, "更新后的摘要", string(IntentDecision)); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	got2, err := repo.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if got2.Summary != "更新后的摘要" {
		t.Fatalf("unexpected updated summary: %s", got2.Summary)
	}
	if got2.LastIntent != string(IntentDecision) {
		t.Fatalf("unexpected updated intent: %s", got2.LastIntent)
	}
}
