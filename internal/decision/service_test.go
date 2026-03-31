package decision

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"genFu/internal/agent"
	"genFu/internal/db"
	"genFu/internal/generate"
	"genFu/internal/investment"
	"genFu/internal/message"
	stockpicker "genFu/internal/stockpicker"
	"genFu/internal/testutil"
	"genFu/internal/tool"
	"genFu/internal/trade_signal"
)

type fakeAgent struct{}

func (f fakeAgent) Name() string           { return "fake" }
func (f fakeAgent) Capabilities() []string { return nil }
func (f fakeAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	_ = req
	raw := `{"decision_id":"d1","decisions":[{"account_id":1,"symbol":"AAPL","action":"buy","quantity":1,"price":10,"confidence":0.9,"reason":"alpha"}]}`
	toolResults, _ := json.Marshal([]tool.ToolResult{{Name: "echo", Output: "ok"}})
	return generate.GenerateResponse{
		Message: message.Message{Role: message.RoleAssistant, Content: raw},
		Meta:    map[string]string{"tool_results": string(toolResults)},
	}, nil
}

type invalidPlannerAgent struct{}

func (f invalidPlannerAgent) Name() string           { return "invalid_planner" }
func (f invalidPlannerAgent) Capabilities() []string { return nil }
func (f invalidPlannerAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	_ = req
	return generate.GenerateResponse{
		Message: message.Message{Role: message.RoleAssistant, Content: `not json`},
	}, nil
}

type fakeTool struct{}

func (f fakeTool) Spec() tool.ToolSpec {
	return tool.ToolSpec{Name: "echo"}
}
func (f fakeTool) Execute(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	_ = ctx
	return tool.ToolResult{Name: "echo", Output: args["text"]}, nil
}

type fakeEngine struct{}

func (f fakeEngine) Execute(ctx context.Context, signals []trade_signal.TradeSignal) ([]trade_signal.ExecutionResult, error) {
	_ = ctx
	results := make([]trade_signal.ExecutionResult, 0, len(signals))
	for _, s := range signals {
		results = append(results, trade_signal.ExecutionResult{Signal: s, Status: "executed"})
	}
	return results, nil
}

type captureAgent struct {
	lastInput string
}

func (c *captureAgent) Name() string           { return "capture" }
func (c *captureAgent) Capabilities() []string { return nil }
func (c *captureAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	if len(req.Messages) > 0 {
		c.lastInput = req.Messages[0].Content
	}
	raw := `{"decision_id":"d2","decisions":[{"account_id":1,"symbol":"600519","action":"hold","quantity":1,"price":1,"confidence":0.9,"valid_until":"2030-01-01T00:00:00Z","reason":"ok"}]}`
	return generate.GenerateResponse{
		Message: message.Message{Role: message.RoleAssistant, Content: raw},
	}, nil
}

func setupDecisionTestRepos(t *testing.T) (*investment.Repository, *stockpicker.GuideRepository, int64, int64) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "decision-test.db")
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
		t.Fatalf("open db: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := db.ApplyMigrations(context.Background(), conn); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	investRepo := investment.NewRepository(conn)
	guideRepo := stockpicker.NewGuideRepository(conn)
	ctx := context.Background()
	user, err := investRepo.CreateUser(ctx, "decision-user")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := investRepo.CreateAccount(ctx, user.ID, "main", "CNY")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	instrument, err := investRepo.UpsertInstrument(ctx, "600519", "贵州茅台", "stock")
	if err != nil {
		t.Fatalf("upsert instrument: %v", err)
	}
	if _, err := investRepo.SetPosition(ctx, account.ID, instrument.ID, 1, 1000, nil); err != nil {
		t.Fatalf("set position: %v", err)
	}

	return investRepo, guideRepo, account.ID, instrument.ID
}

func TestDecide(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	reg := tool.NewRegistry()
	reg.Register(fakeTool{})
	svc := NewService(fakeAgent{}, reg, fakeEngine{}, nil, nil, nil, nil)
	resp, err := svc.Decide(context.Background(), DecisionRequest{AccountID: cfg.News.AccountID})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if resp.Decision.DecisionID != "d1" {
		t.Fatalf("unexpected decision id")
	}
	if len(resp.Signals) != 1 || resp.Signals[0].Symbol != "AAPL" {
		t.Fatalf("unexpected signals")
	}
	if len(resp.ToolResults) != 1 {
		t.Fatalf("expected tool results")
	}
	if len(resp.Executions) != 1 || resp.Executions[0].Status != "executed" {
		t.Fatalf("unexpected executions")
	}
	if len(resp.PlannedOrders) != 1 {
		t.Fatalf("expected planned orders")
	}
	if len(resp.GuardedOrders) != 1 || resp.GuardedOrders[0].GuardStatus != "approved" {
		t.Fatalf("expected approved guarded orders")
	}
	if resp.RiskBudget.MinConfidence != DefaultRiskBudget().MinConfidence {
		t.Fatalf("unexpected risk budget default")
	}
	_ = agent.Agent(fakeAgent{})
}

func TestDecide_WithRiskMetaOverrideBlocksOrder(t *testing.T) {
	reg := tool.NewRegistry()
	svc := NewService(fakeAgent{}, reg, fakeEngine{}, nil, nil, nil, nil)
	resp, err := svc.Decide(context.Background(), DecisionRequest{
		AccountID: 1,
		Meta: map[string]string{
			"risk.min_confidence": "0.95",
		},
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if len(resp.GuardedOrders) != 1 || resp.GuardedOrders[0].GuardStatus != "blocked" {
		t.Fatalf("expected blocked order")
	}
	if len(resp.Executions) != 1 || resp.Executions[0].Status != "blocked" {
		t.Fatalf("expected blocked execution")
	}
}

func TestDecide_WithInvalidPlannerOutputReturnsError(t *testing.T) {
	reg := tool.NewRegistry()
	svc := NewService(
		fakeAgent{},
		reg,
		fakeEngine{},
		nil,
		nil,
		nil,
		nil,
		WithExecutionPlannerAgent(invalidPlannerAgent{}),
	)
	_, err := svc.Decide(context.Background(), DecisionRequest{AccountID: 1})
	if err == nil {
		t.Fatalf("expected planner parse error")
	}
	if !strings.Contains(err.Error(), "invalid_planner_json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecide_WithGuideSelectionsPersistsDefaultGuide(t *testing.T) {
	investRepo, guideRepo, accountID, instrumentID := setupDecisionTestRepos(t)
	ctx := context.Background()

	guide := &stockpicker.OperationGuide{
		Symbol:            "600519",
		PickID:            "pick_1",
		BuyConditions:     []stockpicker.Condition{{Type: "text", Description: "买入A"}},
		SellConditions:    []stockpicker.Condition{{Type: "text", Description: "卖出A"}},
		TradeGuideText:    "guide-text",
		TradeGuideJSON:    `{"asset_type":"stock","symbol":"600519"}`,
		TradeGuideVersion: "v1",
	}
	if err := guideRepo.SaveGuide(ctx, guide); err != nil {
		t.Fatalf("save guide: %v", err)
	}

	capture := &captureAgent{}
	reg := tool.NewRegistry()
	svc := NewService(capture, reg, fakeEngine{}, nil, investRepo, guideRepo, nil)

	_, err := svc.Decide(ctx, DecisionRequest{
		AccountID: accountID,
		GuideSelections: []GuideSelection{
			{Symbol: "600519", GuideID: guide.ID},
		},
	})
	if err != nil {
		t.Fatalf("decide with guide selections: %v", err)
	}

	position, err := investRepo.GetPosition(ctx, accountID, instrumentID)
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if position.OperationGuideID == nil || *position.OperationGuideID != guide.ID {
		t.Fatalf("operation_guide_id not persisted, got=%v want=%d", position.OperationGuideID, guide.ID)
	}
	if !strings.Contains(capture.lastInput, "selected_trade_guides") {
		t.Fatalf("decision input should contain selected_trade_guides, got: %s", capture.lastInput)
	}
}

func TestDecide_WithInvalidGuideSelectionReturnsError(t *testing.T) {
	investRepo, guideRepo, accountID, _ := setupDecisionTestRepos(t)
	reg := tool.NewRegistry()
	svc := NewService(fakeAgent{}, reg, fakeEngine{}, nil, investRepo, guideRepo, nil)

	_, err := svc.Decide(context.Background(), DecisionRequest{
		AccountID: accountID,
		GuideSelections: []GuideSelection{
			{Symbol: "600519", GuideID: 999999},
		},
	})
	if err == nil {
		t.Fatalf("expected error for invalid guide selection")
	}
}
