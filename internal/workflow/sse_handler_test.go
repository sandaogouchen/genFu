//go:build live

package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"genFu/internal/agent/bear"
	"genFu/internal/agent/bull"
	"genFu/internal/agent/debate"
	"genFu/internal/agent/summary"
	"genFu/internal/testutil"
	"genFu/internal/tool"
)

func TestStockSSEHandler(t *testing.T) {
	if os.Getenv("GENFU_LIVE_TESTS") == "" {
		t.Skip("skip live test")
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
	cfg := testutil.LoadConfig(t)
	model := localSummaryModel{}
	reg := tool.NewRegistry()
	reg.Register(tool.NewEastMoneyTool())
	routes := cfg.RSSHub.Routes
	if len(routes) == 0 {
		if envRoute := os.Getenv("GENFU_RSSHUB_ROUTE"); envRoute != "" {
			routes = []string{envRoute}
		} else {
			t.Skip("missing rss routes")
		}
	}
	reg.Register(tool.NewRSSHubTool(cfg.RSSHub.BaseURL, routes, cfg.RSSHub.Timeout))
	bullAgent, err := bull.New(model, nil)
	if err != nil {
		t.Fatalf("bull: %v", err)
	}
	bearAgent, err := bear.New(model, nil)
	if err != nil {
		t.Fatalf("bear: %v", err)
	}
	debateAgent, err := debate.New(model, nil)
	if err != nil {
		t.Fatalf("debate: %v", err)
	}
	summaryAgent, err := summary.New(model, nil)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	wf, err := newStockWorkflowWithAgents(context.Background(), model, reg, nil, routes, bullAgent, bearAgent, debateAgent, summaryAgent)
	if err != nil {
		t.Fatalf("workflow: %v", err)
	}
	symbol := os.Getenv("GENFU_STOCK_SYMBOL")
	if symbol == "" {
		symbol = "000001"
	}
	reqBody, err := json.Marshal(StockWorkflowInput{
		Symbol:             symbol,
		Name:               symbol,
		StockNewsRoutes:    routes,
		IndustryNewsRoutes: routes,
		NewsLimit:          5,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	handler := NewStockSSEHandler(wf)
	req := httptest.NewRequest(http.MethodPost, "/sse/workflow/stock", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	log.Printf("body: %s", body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	raw := string(body)
	if !strings.Contains(raw, "event: holdings") {
		t.Fatalf("missing holdings event")
	}
	if !strings.Contains(raw, "event: news_summary") {
		t.Skip("no news summary event")
	}
	if !strings.Contains(raw, "event: summary") {
		t.Fatalf("missing summary event")
	}
	if !strings.Contains(raw, "event: complete") {
		t.Fatalf("missing complete event")
	}
}
