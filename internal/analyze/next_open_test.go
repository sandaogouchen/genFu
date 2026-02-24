package analyze

import (
	"context"
	"encoding/json"
	"genFu/internal/config"
	"genFu/internal/db"
	"genFu/internal/investment"
	"genFu/internal/llm"
	"genFu/internal/testutil"
	"genFu/internal/tool"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseNextOpenGuideOutputJSON(t *testing.T) {
	raw := `{"brief":"简报","guide":"指南"}`
	parsed, ok := parseNextOpenGuideOutput(raw)
	if !ok {
		t.Fatalf("expected ok")
	}
	if parsed.Brief != "简报" {
		t.Fatalf("unexpected brief: %s", parsed.Brief)
	}
	if parsed.Guide != "指南" {
		t.Fatalf("unexpected guide: %s", parsed.Guide)
	}
}

func TestParseNextOpenGuideOutputFence(t *testing.T) {
	raw := "```json\n{\"brief\":\"A\",\"guide\":\"B\"}\n```"
	parsed, ok := parseNextOpenGuideOutput(raw)
	if !ok {
		t.Fatalf("expected ok")
	}
	if parsed.Brief != "A" || parsed.Guide != "B" {
		t.Fatalf("unexpected output: %+v", parsed)
	}
}

func TestParseNextOpenGuideOutputInvalid(t *testing.T) {
	raw := "not json"
	_, ok := parseNextOpenGuideOutput(raw)
	if ok {
		t.Fatalf("expected false")
	}
}

func TestNextOpenGuideSchedulerNextRun(t *testing.T) {
	loc := time.UTC
	scheduler := NewNextOpenGuideScheduler(nil, 9, 30, loc)
	now := time.Date(2025, 1, 2, 8, 0, 0, 0, loc)
	next := scheduler.nextRun(now)
	expected := time.Date(2025, 1, 2, 9, 30, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("unexpected next run: %s", next)
	}
	now = time.Date(2025, 1, 2, 10, 0, 0, 0, loc)
	next = scheduler.nextRun(now)
	expected = time.Date(2025, 1, 3, 9, 30, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("unexpected next run after cutoff: %s", next)
	}
}

func TestNextOpenGuideLive(t *testing.T) {
	// if os.Getenv("GENFU_LIVE_TESTS") == "" {
	// 	t.Skip("skip live test")
	// }
	path, err := testutil.ConfigPath()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	appConfig, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	dsn := appConfig.PG.DSN
	if override := os.Getenv("GENFU_DB_DSN"); override != "" {
		dsn = override
	}
	log.Printf("使用直连LLM endpoint=%s model=%s", appConfig.LLM.Endpoint, appConfig.LLM.Model)
	llmClient, err := llm.NewEinoChatModel(appConfig.LLM)
	if err != nil {
		t.Fatalf("llm client: %v", err)
	}
	dbConfig := db.Config{
		DSN:             dsn,
		MaxOpenConns:    appConfig.PG.MaxOpenConns,
		MaxIdleConns:    appConfig.PG.MaxIdleConns,
		ConnMaxLifetime: appConfig.PG.ConnMaxLifetime,
	}
	database, err := db.Open(dbConfig)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	if err := chdirRepoRoot(); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}
	if err := pingWithRetry(database, 3, 2*time.Second); err != nil {
		t.Fatalf("db ping: %v", err)
	}
	if err := db.ApplyMigrations(context.Background(), database); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	registry := tool.NewRegistry()
	registry.Register(tool.NewEastMoneyTool())
	registry.Register(tool.NewRSSHubTool(appConfig.RSSHub.BaseURL, appConfig.RSSHub.Routes, appConfig.RSSHub.Timeout))
	analyzeRepo := NewRepository(database)
	investmentRepo := investment.NewRepository(database)
	service := NewNextOpenGuideService(llmClient, registry, analyzeRepo, investmentRepo, appConfig.NextOpen.AccountID, appConfig.RSSHub.Routes, appConfig.NextOpen.NewsLimit)
	id, err := service.Run(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "status=404") {
			t.Skip("llm endpoint unavailable")
		}
		t.Fatalf("run: %v", err)
	}
	if id == 0 {
		t.Fatalf("empty report id")
	}
	report, err := analyzeRepo.GetReport(context.Background(), id)
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if report.Summary == "" {
		t.Fatalf("empty summary")
	}
	var output map[string]string
	if err := json.Unmarshal([]byte(report.Summary), &output); err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	log.Printf("brief: %s", output["brief"])
	log.Printf("guide: %s", output["guide"])
	if strings.TrimSpace(output["brief"]) == "" {
		t.Fatalf("empty brief")
	}
	if strings.TrimSpace(output["guide"]) == "" {
		t.Fatalf("empty guide")
	}
}
