package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/db"
	decisionpkg "genFu/internal/decision"
	"genFu/internal/generate"
	"genFu/internal/message"
	stockpickerpkg "genFu/internal/stockpicker"
	"genFu/internal/testutil"
	"genFu/internal/tool"
	"genFu/internal/trade_signal"
)

type fixedRouter struct {
	decision RouteDecision
}

func (r fixedRouter) Route(ctx context.Context, input RouteInput) (RouteDecision, error) {
	_ = ctx
	_ = input
	return r.decision, nil
}

type capturingChatModel struct {
	mu        sync.Mutex
	lastTools []string
	content   string
}

func (m *capturingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		names = append(names, t.Name)
	}
	m.mu.Lock()
	m.lastTools = names
	m.mu.Unlock()
	return m, nil
}

func (m *capturingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	_ = ctx
	_ = input
	_ = opts
	content := m.content
	if strings.TrimSpace(content) == "" {
		content = "ok"
	}
	return schema.AssistantMessage(content, nil), nil
}

func (m *capturingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	_ = ctx
	_ = input
	_ = opts
	reader, writer := schema.Pipe[*schema.Message](2)
	go func() {
		writer.Send(schema.AssistantMessage("o", nil), nil)
		writer.Send(schema.AssistantMessage("k", nil), nil)
		writer.Close()
	}()
	return reader, nil
}

func (m *capturingChatModel) tools() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.lastTools))
	copy(out, m.lastTools)
	return out
}

type fixedMemoryAgent struct {
	mu         sync.Mutex
	summaries  []string
	prevInputs []string
}

func (a *fixedMemoryAgent) Summarize(ctx context.Context, previousSummary string, transcript []message.Message) (string, error) {
	_ = ctx
	_ = transcript
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prevInputs = append(a.prevInputs, previousSummary)
	if len(a.summaries) == 0 {
		return previousSummary, nil
	}
	next := a.summaries[0]
	a.summaries = a.summaries[1:]
	return next, nil
}

type fakeDecisionWorkflow struct {
	mu      sync.Mutex
	called  int
	lastReq decisionpkg.DecisionRequest
}

func (f *fakeDecisionWorkflow) Decide(ctx context.Context, req decisionpkg.DecisionRequest) (decisionpkg.DecisionResponse, error) {
	_ = ctx
	f.mu.Lock()
	f.called++
	f.lastReq = req
	f.mu.Unlock()
	return decisionpkg.DecisionResponse{
		Decision: trade_signal.DecisionOutput{DecisionID: "d-1", MarketView: "neutral"},
	}, nil
}

type fakeStockpickerWorkflow struct {
	mu      sync.Mutex
	called  int
	lastReq stockpickerpkg.StockPickRequest
}

func (f *fakeStockpickerWorkflow) PickStocks(ctx context.Context, req stockpickerpkg.StockPickRequest) (stockpickerpkg.StockPickResponse, error) {
	_ = ctx
	f.mu.Lock()
	f.called++
	f.lastReq = req
	f.mu.Unlock()
	return stockpickerpkg.StockPickResponse{
		PickID:      "p-1",
		GeneratedAt: time.Now(),
		Stocks:      []stockpickerpkg.StockPick{},
	}, nil
}

type fakeNamedTool struct{ name string }

func (t fakeNamedTool) Spec() tool.ToolSpec {
	return tool.ToolSpec{Name: t.name}
}

func (t fakeNamedTool) Execute(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	_ = ctx
	_ = args
	return tool.ToolResult{Name: t.name, Output: "ok"}, nil
}

func TestChatStreamEmitsIntentBeforeContent(t *testing.T) {
	conn := setupChatTestDB(t)
	repo := NewRepository(conn)
	model := &capturingChatModel{content: "chat"}
	decisionSvc := &fakeDecisionWorkflow{}
	svc := NewService(
		model,
		repo,
		tool.NewRegistry(),
		WithDecisionService(decisionSvc),
		WithIntentRouter(fixedRouter{decision: RouteDecision{
			Intent:       IntentDecision,
			Workflow:     WorkflowDomainDecision,
			Confidence:   0.95,
			AllowedTools: []string{},
		}}),
		WithSessionMemoryAgent(&fixedMemoryAgent{summaries: []string{"m1"}}),
	)
	ch, _, err := svc.ChatStream(context.Background(), generate.GenerateRequest{
		SessionID: "s-intent",
		Messages:  []message.Message{{Role: message.RoleUser, Content: "请给我交易建议"}},
	})
	if err != nil {
		t.Fatalf("chat stream: %v", err)
	}
	evt1 := <-ch
	evt2 := <-ch
	if evt1.Type != "session" {
		t.Fatalf("expected first event session, got %s", evt1.Type)
	}
	if evt2.Type != "intent" {
		t.Fatalf("expected second event intent, got %s", evt2.Type)
	}
	if evt2.Intent == nil || evt2.Intent.Intent != string(IntentDecision) {
		t.Fatalf("unexpected intent payload: %#v", evt2.Intent)
	}
}

func TestChatServiceDispatchAndWhitelist(t *testing.T) {
	conn := setupChatTestDB(t)
	repo := NewRepository(conn)
	reg := tool.NewRegistry()
	reg.Register(fakeNamedTool{name: "investment"})
	reg.Register(fakeNamedTool{name: "eastmoney"})

	model := &capturingChatModel{content: "chat"}
	decisionSvc := &fakeDecisionWorkflow{}
	stockpickerSvc := &fakeStockpickerWorkflow{}
	mem := &fixedMemoryAgent{summaries: []string{"m1", "m2", "m3", "m4"}}

	svc := NewService(
		model,
		repo,
		reg,
		WithDecisionService(decisionSvc),
		WithStockPickerService(stockpickerSvc),
		WithSessionMemoryAgent(mem),
	)

	// general_chat -> chat workflow, no tools
	svc.intentRouter = fixedRouter{decision: RouteDecision{
		Intent:       IntentGeneralChat,
		Workflow:     WorkflowChatGeneral,
		Confidence:   0.99,
		AllowedTools: []string{},
	}}
	_, _, err := svc.Chat(context.Background(), generate.GenerateRequest{
		SessionID: "s-dispatch",
		Messages:  []message.Message{{Role: message.RoleUser, Content: "你好"}},
	})
	if err != nil {
		t.Fatalf("general chat: %v", err)
	}
	if len(model.tools()) != 0 {
		t.Fatalf("general chat should not expose tools: %#v", model.tools())
	}

	// portfolio_ops -> whitelist intersection (investment only)
	svc.intentRouter = fixedRouter{decision: RouteDecision{
		Intent:       IntentPortfolio,
		Workflow:     WorkflowChatPortfolio,
		Confidence:   0.99,
		AllowedTools: []string{"investment"},
	}}
	_, _, err = svc.Chat(context.Background(), generate.GenerateRequest{
		SessionID: "s-dispatch",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: "帮我看看持仓"},
		},
		Tools: []tool.ToolSpec{
			{Name: "eastmoney"},
			{Name: "investment"},
		},
	})
	if err != nil {
		t.Fatalf("portfolio chat: %v", err)
	}
	tools := model.tools()
	if len(tools) != 1 || tools[0] != "investment" {
		t.Fatalf("unexpected tools for portfolio: %#v", tools)
	}

	// decision -> domain workflow, meta overrides slots
	svc.intentRouter = fixedRouter{decision: RouteDecision{
		Intent:       IntentDecision,
		Workflow:     WorkflowDomainDecision,
		Confidence:   0.99,
		AllowedTools: []string{},
		Slots: RouteSlots{
			AccountID: 1,
			ReportIDs: []int64{7, 8},
		},
	}}
	_, _, err = svc.Chat(context.Background(), generate.GenerateRequest{
		SessionID: "s-dispatch",
		Messages:  []message.Message{{Role: message.RoleUser, Content: "给我决策"}},
		Meta: map[string]string{
			"account_id": "9",
			"report_ids": "2,3,0",
		},
	})
	if err != nil {
		t.Fatalf("decision workflow: %v", err)
	}
	if decisionSvc.lastReq.AccountID != 9 {
		t.Fatalf("decision account_id not overridden: %d", decisionSvc.lastReq.AccountID)
	}
	if len(decisionSvc.lastReq.ReportIDs) != 2 || decisionSvc.lastReq.ReportIDs[0] != 2 {
		t.Fatalf("decision report_ids not applied: %#v", decisionSvc.lastReq.ReportIDs)
	}

	// stockpicker -> domain workflow with top_n clamp
	svc.intentRouter = fixedRouter{decision: RouteDecision{
		Intent:       IntentStockPicker,
		Workflow:     WorkflowDomainPicker,
		Confidence:   0.99,
		AllowedTools: []string{},
		Slots: RouteSlots{
			TopN:     30,
			DateFrom: "2026-02-10",
			DateTo:   "2026-02-11",
		},
	}}
	_, _, err = svc.Chat(context.Background(), generate.GenerateRequest{
		SessionID: "s-dispatch",
		Messages:  []message.Message{{Role: message.RoleUser, Content: "选股"}},
		Meta: map[string]string{
			"top_n":      "50",
			"account_id": "3",
		},
	})
	if err != nil {
		t.Fatalf("stockpicker workflow: %v", err)
	}
	if stockpickerSvc.lastReq.TopN != 20 {
		t.Fatalf("expected top_n clamp to 20, got %d", stockpickerSvc.lastReq.TopN)
	}
	if stockpickerSvc.lastReq.AccountID != 3 {
		t.Fatalf("unexpected stockpicker account id: %d", stockpickerSvc.lastReq.AccountID)
	}
}

func TestChatServiceMemoryAcrossTurns(t *testing.T) {
	conn := setupChatTestDB(t)
	repo := NewRepository(conn)
	model := &capturingChatModel{content: "chat"}
	memAgent := &fixedMemoryAgent{summaries: []string{"summary-1", "summary-2"}}
	svc := NewService(
		model,
		repo,
		tool.NewRegistry(),
		WithIntentRouter(fixedRouter{decision: RouteDecision{
			Intent:       IntentGeneralChat,
			Workflow:     WorkflowChatGeneral,
			Confidence:   0.99,
			AllowedTools: []string{},
		}}),
		WithSessionMemoryAgent(memAgent),
	)
	ctx := context.Background()
	_, sessionID, err := svc.Chat(ctx, generate.GenerateRequest{
		SessionID: "s-memory",
		Messages:  []message.Message{{Role: message.RoleUser, Content: "第一轮"}},
	})
	if err != nil {
		t.Fatalf("chat1: %v", err)
	}
	_, _, err = svc.Chat(ctx, generate.GenerateRequest{
		SessionID: sessionID,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: "第一轮"},
			{Role: message.RoleAssistant, Content: "chat"},
			{Role: message.RoleUser, Content: "第二轮"},
		},
	})
	if err != nil {
		t.Fatalf("chat2: %v", err)
	}
	memRepo := NewSessionMemoryRepository(conn)
	got, err := memRepo.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("memory get: %v", err)
	}
	if got.Summary != "summary-2" {
		t.Fatalf("unexpected summary: %s", got.Summary)
	}
	if len(memAgent.prevInputs) < 2 || memAgent.prevInputs[1] != "summary-1" {
		t.Fatalf("expected previous summary propagation, got %#v", memAgent.prevInputs)
	}
}

func setupChatTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "chat-orchestration.db")
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
	return conn
}
