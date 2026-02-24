package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/schema"

	"genFu/internal/db"
	"genFu/internal/testutil"
)

func TestRepositorySessionMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.db")
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
	repo := NewRepository(conn)
	ctx := context.Background()
	sessionID, err := repo.EnsureSession(ctx, "", "u1")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if sessionID == "" {
		t.Fatalf("missing session id")
	}
	msgs := []*schema.Message{
		schema.UserMessage("hi"),
		schema.AssistantMessage("hello", nil),
	}
	if err := repo.AppendMessages(ctx, sessionID, msgs); err != nil {
		t.Fatalf("append: %v", err)
	}
	stored, err := repo.ListMessages(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(stored))
	}
	if stored[0].Content != "hi" || stored[1].Content != "hello" {
		t.Fatalf("unexpected message content")
	}
}
