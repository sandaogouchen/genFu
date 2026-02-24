package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"genFu/internal/testutil"
)
func TestOpenAndPing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	cfg := testutil.LoadConfig(t)
	dbCfg := cfg.PG
	dbCfg.DSN = "file:" + path
	conn, err := Open(Config{
		DSN:             dbCfg.DSN,
		MaxOpenConns:    dbCfg.MaxOpenConns,
		MaxIdleConns:    dbCfg.MaxIdleConns,
		ConnMaxLifetime: dbCfg.ConnMaxLifetime,
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestPingNil(t *testing.T) {
	var conn *DB
	if err := conn.Ping(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestApplyMigrations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	cfg := testutil.LoadConfig(t)
	dbCfg := cfg.PG
	dbCfg.DSN = "file:" + path
	conn, err := Open(Config{
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
	ctx := context.Background()
	if err := ApplyMigrations(ctx, conn); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	var count int
	if err := conn.QueryRowContext(ctx, "select count(1) from schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected migrations applied")
	}
}
