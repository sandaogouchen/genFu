package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/db"
	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/testutil"
	"genFu/internal/tool"
)

type fakeChatModel struct{}

func (m *fakeChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *fakeChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	_ = ctx
	_ = input
	_ = opts
	return schema.AssistantMessage("hello", nil), nil
}

func (m *fakeChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	_ = ctx
	_ = input
	_ = opts
	reader, writer := schema.Pipe[*schema.Message](4)
	go func() {
		writer.Send(schema.AssistantMessage("he", nil), nil)
		writer.Send(schema.AssistantMessage("llo", nil), nil)
		writer.Close()
	}()
	return reader, nil
}

func TestChatHandler(t *testing.T) {
	service := newTestService(t)
	handler := NewHandler(service)
	reqBody, err := json.Marshal(generate.GenerateRequest{
		SessionID: "s",
		Messages:  []message.Message{{Role: message.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	raw := string(body)
	if !strings.Contains(raw, "event: session") {
		t.Fatalf("missing session event")
	}
	if !strings.Contains(raw, "event: done") {
		t.Fatalf("missing done event")
	}
}

func TestChatSSEHandler(t *testing.T) {
	service := newTestService(t)
	handler := NewSSEHandler(service)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	reqBody, err := json.Marshal(generate.GenerateRequest{
		SessionID: "s",
		Messages:  []message.Message{{Role: message.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	raw := string(body)
	if !strings.Contains(raw, "event: session") {
		t.Fatalf("missing session event")
	}
	if !strings.Contains(raw, "event: done") {
		t.Fatalf("missing done event")
	}
}

func newTestService(t *testing.T) *Service {
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
	return NewService(&fakeChatModel{}, repo, tool.NewRegistry())
}
