package analyze

import (
	"context"
	"genFu/internal/config"
	"genFu/internal/db"
	"genFu/internal/investment"
	"genFu/internal/tool"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genFu/internal/llm"
	"genFu/internal/testutil"
)

func TestDailyReviewLive(t *testing.T) {
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
	analyzeRepo := NewRepository(database)
	investmentRepo := investment.NewRepository(database)
	service := NewDailyReviewService(llmClient, registry, analyzeRepo, investmentRepo)
	ctx := context.WithValue(context.Background(), "now", time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC))
	id, err := service.Run(ctx)
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
	log.Printf("report: %v", report)
}

func pingWithRetry(database *db.DB, attempts int, delay time.Duration) error {
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		lastErr = database.Ping(context.Background())
		if lastErr == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return lastErr
}

func chdirRepoRoot() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	return os.Chdir(root)
}
