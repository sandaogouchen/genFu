//go:build live

package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"genFu/internal/db"
	"genFu/internal/investment"
	"genFu/internal/llm"
	"genFu/internal/rsshub"
	"genFu/internal/testutil"
	"genFu/internal/tool"
)

func TestStockSSEFullChain(t *testing.T) {
	// if os.Getenv("GENFU_LIVE_TESTS") == "" {
	// 	t.Skip("skip live test")
	// }
	log.Printf("start TestStockSSEFullChain")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("wd: %v", err)
	}
	log.Printf("cwd: %s", wd)
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	cfg := testutil.LoadConfig(t)
	baseURL := cfg.RSSHub.BaseURL
	log.Printf("rsshub base_url: %s", baseURL)
	model, err := llm.NewEinoChatModel(cfg.LLM)
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	log.Printf("llm model: %s", cfg.LLM.Model)
	log.Printf("llm endpoint: %s", cfg.LLM.Endpoint)
	symbol := os.Getenv("GENFU_STOCK_SYMBOL")
	if symbol == "" {
		symbol = "000001"
	}
	log.Printf("symbol: %s", symbol)
	routes := cfg.RSSHub.Routes
	if envRoute := os.Getenv("GENFU_RSSHUB_ROUTE"); envRoute != "" {
		routes = []string{envRoute}
	}
	if len(routes) == 0 {
		t.Fatalf("missing rss routes")
	}
	log.Printf("rsshub routes: %v", routes)
	selectedRoute, err := pickRSSRoute(baseURL, routes)
	if err != nil {
		t.Fatalf("rss route: %v", err)
	}
	log.Printf("rsshub selected route: %s", selectedRoute)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "live.db")
	dbCfg := cfg.PG
	dbCfg.DSN = "file:" + dbPath
	log.Printf("db dsn: %s", dbCfg.DSN)
	conn, err := db.Open(db.Config{
		DSN:             dbCfg.DSN,
		MaxOpenConns:    dbCfg.MaxOpenConns,
		MaxIdleConns:    dbCfg.MaxIdleConns,
		ConnMaxLifetime: dbCfg.ConnMaxLifetime,
	})
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	log.Printf("db open ok")
	if err := db.ApplyMigrations(context.Background(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	log.Printf("db migrations ok")
	investRepo := investment.NewRepository(conn)
	user, err := investRepo.CreateUser(context.Background(), "live")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	log.Printf("user created: %d", user.ID)
	account, err := investRepo.CreateAccount(context.Background(), user.ID, "live", "CNY")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	log.Printf("account created: %d", account.ID)
	instrument, err := investRepo.UpsertInstrument(context.Background(), symbol, symbol, "stock")
	if err != nil {
		t.Fatalf("upsert instrument: %v", err)
	}
	log.Printf("instrument created: %d %s", instrument.ID, instrument.Symbol)
	if _, err := investRepo.SetPosition(context.Background(), account.ID, instrument.ID, 100, 10, nil); err != nil {
		t.Fatalf("set position: %v", err)
	}
	log.Printf("position set")
	reg := tool.NewRegistry()
	reg.Register(tool.NewEastMoneyTool())
	reg.Register(tool.NewRSSHubTool(baseURL, []string{selectedRoute}, cfg.RSSHub.Timeout))
	log.Printf("tools registered")
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
	log.Printf("agents initialized")
	wf, err := newStockWorkflowWithAgents(context.Background(), model, reg, investRepo, bullAgent, bearAgent, debateAgent, summaryAgent)
	if err != nil {
		t.Fatalf("workflow: %v", err)
	}
	log.Printf("workflow compiled")
	reqBody, err := json.Marshal(StockWorkflowInput{
		AccountID:          account.ID,
		Symbol:             symbol,
		Name:               symbol,
		StockNewsRoutes:    []string{selectedRoute},
		IndustryNewsRoutes: []string{selectedRoute},
		NewsLimit:          5,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	log.Printf("request body size: %d", len(reqBody))
	handler := NewStockSSEHandler(wf)
	req := httptest.NewRequest(http.MethodPost, "/sse/workflow/stock", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	log.Printf("invoke sse handler")
	handler.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	log.Printf("response size: %d", len(body))
	log.Printf("body: %s", body)
	raw := string(body)
	if strings.Contains(raw, "event: error") {
		errData := getEventData(t, raw, "error")
		t.Fatalf("workflow error: %s", string(errData))
	}
	assertEvent(t, raw, "holdings")
	assertEvent(t, raw, "holdings_market")
	assertEvent(t, raw, "target_market")
	assertEvent(t, raw, "news_summary")
	assertEvent(t, raw, "bull")
	assertEvent(t, raw, "bear")
	assertEvent(t, raw, "debate")
	assertEvent(t, raw, "summary")
	assertEvent(t, raw, "complete")
	holdingsData := getEventData(t, raw, "holdings")
	var holdings HoldingsOutput
	if err := json.Unmarshal(holdingsData, &holdings); err != nil {
		t.Fatalf("holdings json: %v", err)
	}
	if len(holdings.Positions) == 0 {
		t.Fatalf("missing holdings positions")
	}
	marketData := getEventData(t, raw, "target_market")
	var target MarketMove
	if err := json.Unmarshal(marketData, &target); err != nil {
		t.Fatalf("target json: %v", err)
	}
	if target.Price <= 0 {
		t.Fatalf("missing target price")
	}
	newsData := getEventData(t, raw, "news_summary")
	var news NewsSummaryOutput
	if err := json.Unmarshal(newsData, &news); err != nil {
		t.Fatalf("news json: %v", err)
	}
	if len(news.Items) == 0 {
		t.Fatalf("missing news items")
	}
	if strings.TrimSpace(news.Summary) == "" || strings.TrimSpace(news.Sentiment) == "" {
		t.Fatalf("missing news summary")
	}
}

func assertEvent(t *testing.T, body string, event string) {
	t.Helper()
	if !strings.Contains(body, "event: "+event) {
		t.Fatalf("missing event: %s", event)
	}
}

func pickRSSRoute(baseURL string, routes []string) (string, error) {
	client := rsshub.NewClient(baseURL, 0)
	for _, route := range routes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}
		items, err := client.Fetch(route, 5)
		if err != nil {
			continue
		}
		if len(items) > 0 {
			log.Printf("rsshub first item: %s %s", items[0].Title, items[0].Link)
			return route, nil
		}
	}
	return "", fmt.Errorf("no_rss_items base_url=%s routes=%v", baseURL, routes)
}

func getEventData(t *testing.T, body string, event string) []byte {
	t.Helper()
	token := "event: " + event + "\n"
	idx := strings.Index(body, token)
	if idx < 0 {
		t.Fatalf("event not found: %s", event)
	}
	rest := body[idx+len(token):]
	if !strings.HasPrefix(rest, "data: ") {
		t.Fatalf("event missing data: %s", event)
	}
	rest = strings.TrimPrefix(rest, "data: ")
	end := strings.Index(rest, "\n\n")
	if end < 0 {
		t.Fatalf("event data not terminated: %s", event)
	}
	return []byte(rest[:end])
}
