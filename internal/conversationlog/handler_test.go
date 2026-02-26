package conversationlog

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"genFu/internal/db"
	"genFu/internal/testutil"
)

func newHandlerRepo(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "conversationlog-handler.db")
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

func TestSessionsAndRunsHandlers(t *testing.T) {
	repo := newHandlerRepo(t)
	sessionsHandler := NewSessionsHandler(repo)
	sessionItemHandler := NewSessionItemHandler(repo)
	runsHandler := NewRunsHandler(repo)

	createBody, _ := json.Marshal(CreateSessionRequest{Scene: SceneAnalyze, Title: "会话1"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/conversations/sessions", bytes.NewReader(createBody))
	createRec := httptest.NewRecorder()
	sessionsHandler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status: %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created Session
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("missing created session id")
	}

	if err := repo.AppendRun(context.Background(), created.ID, "分析股票 600519", json.RawMessage(`{"symbol":"600519"}`), json.RawMessage(`{"summary":"ok"}`), ""); err != nil {
		t.Fatalf("append run: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/conversations/sessions?scene=analyze&limit=20", nil)
	listRec := httptest.NewRecorder()
	sessionsHandler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status: %d body=%s", listRec.Code, listRec.Body.String())
	}

	renameBody := []byte(`{"title":"重命名会话"}`)
	renameReq := httptest.NewRequest(http.MethodPatch, "/api/conversations/sessions/"+created.ID, bytes.NewReader(renameBody))
	renameRec := httptest.NewRecorder()
	sessionItemHandler.ServeHTTP(renameRec, renameReq)
	if renameRec.Code != http.StatusOK {
		t.Fatalf("rename status: %d body=%s", renameRec.Code, renameRec.Body.String())
	}

	runReq := httptest.NewRequest(http.MethodGet, "/api/conversations/runs?session_id="+created.ID, nil)
	runRec := httptest.NewRecorder()
	runsHandler.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("runs status: %d body=%s", runRec.Code, runRec.Body.String())
	}
	body, _ := io.ReadAll(runRec.Result().Body)
	if !bytes.Contains(body, []byte("分析股票 600519")) {
		t.Fatalf("runs payload missing prompt")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/conversations/sessions/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	sessionItemHandler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status: %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	runAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/conversations/runs?session_id="+created.ID, nil)
	runAfterDeleteRec := httptest.NewRecorder()
	runsHandler.ServeHTTP(runAfterDeleteRec, runAfterDeleteReq)
	if runAfterDeleteRec.Code != http.StatusNotFound {
		t.Fatalf("runs after delete status: %d body=%s", runAfterDeleteRec.Code, runAfterDeleteRec.Body.String())
	}
}
