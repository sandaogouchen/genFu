package conversationlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"genFu/internal/db"
	"genFu/internal/testutil"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "conversationlog.db")
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
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := db.ApplyMigrations(context.Background(), conn); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return NewRepository(conn)
}

func TestSessionCRUDAndRuns(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	session, err := repo.CreateSession(ctx, SceneAnalyze, "初始标题", "u1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.ID == "" {
		t.Fatalf("missing session id")
	}
	if session.Scene != SceneAnalyze {
		t.Fatalf("unexpected scene: %s", session.Scene)
	}

	list, err := repo.ListSessions(ctx, SceneAnalyze, 20, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}

	if err := repo.RenameSession(ctx, session.ID, "新标题"); err != nil {
		t.Fatalf("rename session: %v", err)
	}
	renamed, err := repo.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if renamed.Title != "新标题" {
		t.Fatalf("rename not applied: %s", renamed.Title)
	}

	reqJSON, _ := json.Marshal(map[string]interface{}{"symbol": "600519"})
	respJSON, _ := json.Marshal(map[string]interface{}{"summary": "ok"})
	if err := repo.AppendRun(ctx, session.ID, "分析股票 600519", reqJSON, respJSON, ""); err != nil {
		t.Fatalf("append run: %v", err)
	}
	runs, err := repo.ListRuns(ctx, session.ID, 50)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if string(runs[0].Request) == "" {
		t.Fatalf("empty request payload")
	}

	if err := repo.SoftDeleteSession(ctx, session.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := repo.GetSession(ctx, session.ID); err == nil {
		t.Fatalf("expected deleted session to be hidden")
	}
	if _, err := repo.ListRuns(ctx, session.ID, 10); !errorsIsNoRows(err) {
		t.Fatalf("expected not found for deleted session, got %v", err)
	}
}

func TestEnsureSession(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	session, err := repo.EnsureSession(ctx, "", SceneWorkflow, "会话A", "u1")
	if err != nil {
		t.Fatalf("ensure create: %v", err)
	}
	if session.ID == "" {
		t.Fatalf("empty session id")
	}
	again, err := repo.EnsureSession(ctx, session.ID, SceneWorkflow, "", "u1")
	if err != nil {
		t.Fatalf("ensure existing: %v", err)
	}
	if again.ID != session.ID {
		t.Fatalf("expected same session id")
	}
}

func errorsIsNoRows(err error) bool {
	return err == sql.ErrNoRows
}
