package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"genFu/internal/agent"
	"genFu/internal/analyze"
	"genFu/internal/generate"
	"genFu/internal/investment"
	"genFu/internal/message"
	stockpicker "genFu/internal/stockpicker"
	"genFu/internal/tool"
	"genFu/internal/trade_signal"
)

type Service struct {
	agent              agent.Agent
	plannerAgent       agent.Agent
	reviewAgent        agent.Agent
	policyGuard        PolicyGuard
	registry           *tool.Registry
	engine             trade_signal.Engine
	reports            *analyze.Repository
	holdings           *investment.Repository
	guides             *stockpicker.GuideRepository
	provider           MarketNewsProvider
	repo               *Repository
	riskBudgetDefaults RiskBudget
}

type ServiceOption func(*Service)

func WithExecutionPlannerAgent(ag agent.Agent) ServiceOption {
	return func(s *Service) {
		s.plannerAgent = ag
	}
}

func WithPostTradeReviewAgent(ag agent.Agent) ServiceOption {
	return func(s *Service) {
		s.reviewAgent = ag
	}
}

func WithPolicyGuard(pg PolicyGuard) ServiceOption {
	return func(s *Service) {
		s.policyGuard = pg
	}
}

func WithDecisionRepository(repo *Repository) ServiceOption {
	return func(s *Service) {
		s.repo = repo
	}
}

func WithRiskBudgetDefaults(budget RiskBudget) ServiceOption {
	return func(s *Service) {
		s.riskBudgetDefaults = budget.Normalize()
	}
}

func NewService(agent agent.Agent, registry *tool.Registry, engine trade_signal.Engine, reports *analyze.Repository, holdings *investment.Repository, guides *stockpicker.GuideRepository, provider MarketNewsProvider, opts ...ServiceOption) *Service {
	if provider == nil {
		provider = EmptyMarketNewsProvider{}
	}
	s := &Service{
		agent:              agent,
		registry:           registry,
		engine:             engine,
		reports:            reports,
		holdings:           holdings,
		guides:             guides,
		provider:           provider,
		riskBudgetDefaults: DefaultRiskBudget(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.policyGuard == nil {
		s.policyGuard = NewPolicyGuardAgent(holdings)
	}
	return s
}

type decisionGuideInput struct {
	Symbol            string                  `json:"symbol"`
	GuideID           int64                   `json:"guide_id"`
	TradeGuideText    string                  `json:"trade_guide_text,omitempty"`
	TradeGuideJSON    string                  `json:"trade_guide_json,omitempty"`
	TradeGuideVersion string                  `json:"trade_guide_version,omitempty"`
	BuyConditions     []stockpicker.Condition `json:"buy_conditions,omitempty"`
	SellConditions    []stockpicker.Condition `json:"sell_conditions,omitempty"`
	StopLoss          string                  `json:"stop_loss,omitempty"`
	TakeProfit        string                  `json:"take_profit,omitempty"`
	RiskMonitors      []string                `json:"risk_monitors,omitempty"`
	ValidUntil        string                  `json:"valid_until,omitempty"`
}

type plannerOutput struct {
	PlannedOrders []plannerOrderItem `json:"planned_orders"`
}

type plannerOrderItem struct {
	OrderID        string  `json:"order_id"`
	AccountID      int64   `json:"account_id"`
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	AssetType      string  `json:"asset_type"`
	Action         string  `json:"action"`
	Quantity       float64 `json:"quantity"`
	Price          float64 `json:"price"`
	Confidence     float64 `json:"confidence"`
	PlanningReason string  `json:"planning_reason"`
}

type reviewOutput struct {
	Summary        string              `json:"summary"`
	Attributions   []ReviewAttribution `json:"attributions"`
	LearningPoints []string            `json:"learning_points"`
}

func (s *Service) Decide(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	if s == nil {
		return DecisionResponse{}, errors.New("decision_service_not_initialized")
	}
	if s.agent == nil {
		return DecisionResponse{}, errors.New("decision_agent_not_initialized")
	}

	println("\n" + strings.Repeat("=", 80))
	println("[DECISION SERVICE] 开始交易决策流程")
	println(strings.Repeat("=", 80))

	println("\n[DECISION SERVICE] 步骤 1: 加载分析报告")
	reportTexts, err := s.loadReports(ctx, req.ReportIDs)
	if err != nil {
		printf("[DECISION SERVICE] ✗ 加载报告失败: %v\n", err)
		return DecisionResponse{}, err
	}
	printf("[DECISION SERVICE] ✓ 成功加载 %d 份报告\n", len(reportTexts))

	println("\n[DECISION SERVICE] 步骤 2: 确定账户ID")
	accountID := req.AccountID
	if accountID == 0 && s.holdings != nil {
		accountID, err = s.holdings.DefaultAccountID(ctx)
		if err != nil {
			printf("[DECISION SERVICE] ✗ 获取默认账户失败: %v\n", err)
			return DecisionResponse{}, err
		}
	}
	printf("  使用账户ID: %d\n", accountID)

	println("\n[DECISION SERVICE] 步骤 3: 加载持仓和指南")
	selectedGuides, err := s.resolveGuideSelections(ctx, req.GuideSelections)
	if err != nil {
		printf("[DECISION SERVICE] ✗ 解析指南选择失败: %v\n", err)
		return DecisionResponse{}, err
	}
	if err := s.persistGuideSelections(ctx, accountID, selectedGuides); err != nil {
		printf("[DECISION SERVICE] ✗ 写回持仓默认指南失败: %v\n", err)
		return DecisionResponse{}, err
	}
	holdingsText, err := s.loadHoldings(ctx, accountID)
	if err != nil {
		printf("[DECISION SERVICE] ✗ 加载持仓失败: %v\n", err)
		return DecisionResponse{}, err
	}

	println("\n[DECISION SERVICE] 步骤 4: 加载市场和新闻数据")
	marketText, newsText := s.loadMarketNews(ctx)

	println("\n[DECISION SERVICE] 步骤 5: 调用决策Agent")
	input := buildDecisionInput(req, holdingsText, marketText, newsText, reportTexts, selectedGuides)
	resp, err := s.agent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: input}},
	})
	if err != nil {
		printf("[DECISION SERVICE] ✗ 决策Agent处理失败: %v\n", err)
		return DecisionResponse{}, err
	}
	toolResults := parseToolResultsMeta(resp.Meta)

	println("\n[DECISION SERVICE] 步骤 6: 解析决策输出")
	decision, signals, err := trade_signal.ParseDecisionOutput(resp.Message.Content, accountID)
	if err != nil {
		printf("[DECISION SERVICE] ✗ 解析决策输出失败: %v\n", err)
		return DecisionResponse{}, err
	}

	// Step 3.5: [Rule Engine Hook] Load SL/TP rules for position.
	// When rule_engine is enabled, the monitor will independently evaluate
	// stop-loss and take-profit conditions. This hook point is reserved for
	// future inline integration where SL/TP prices from active rules can
	// be attached to trade signals before they are emitted.
	// TODO(rule-engine): Inject SL/TP prices from active rules into signal.

	riskBudget := s.resolveRiskBudget(req.Meta)
	warnings := []string{}
	runID := int64(0)
	runIDText := ""
	if s.repo != nil {
		runID, err = s.repo.CreateRun(ctx, accountID, decision.DecisionID, req, decision, riskBudget)
		if err != nil {
			warnings = appendWarning(warnings, "persist_run_failed: "+err.Error())
		} else {
			runIDText = strconv.FormatInt(runID, 10)
		}
	}

	println("\n[DECISION SERVICE] 步骤 7: 生成执行订单")
	plannedOrders, err := s.planExecution(ctx, accountID, decision, signals, holdingsText, marketText, newsText, reportTexts)
	if err != nil {
		_ = s.finalizeRunWithWarning(ctx, runID, "failed", &warnings)
		printf("[DECISION SERVICE] ✗ 生成执行订单失败: %v\n", err)
		return DecisionResponse{}, err
	}

	println("\n[DECISION SERVICE] 步骤 8: 风控门禁")
	guardedOrders, err := s.guardOrders(ctx, accountID, riskBudget, plannedOrders)
	if err != nil {
		_ = s.finalizeRunWithWarning(ctx, runID, "failed", &warnings)
		printf("[DECISION SERVICE] ✗ 风控门禁失败: %v\n", err)
		return DecisionResponse{}, err
	}

	println("\n[DECISION SERVICE] 步骤 9: 执行订单")
	executions, guardedOrders, err := s.executeGuardedOrders(ctx, decision.DecisionID, guardedOrders)
	if err != nil {
		_ = s.finalizeRunWithWarning(ctx, runID, "failed", &warnings)
		printf("[DECISION SERVICE] ✗ 执行交易失败: %v\n", err)
		return DecisionResponse{}, err
	}

	println("\n[DECISION SERVICE] 步骤 10: 执行后复盘")
	review, reviewErr := s.generateReview(ctx, riskBudget, plannedOrders, guardedOrders, executions, toolResults)
	if reviewErr != nil {
		warnings = appendWarning(warnings, "review_generation_failed: "+reviewErr.Error())
	}

	if runID > 0 && s.repo != nil {
		if err := s.repo.SaveOrders(ctx, runID, guardedOrders); err != nil {
			warnings = appendWarning(warnings, "persist_orders_failed: "+err.Error())
		}
		if err := s.repo.SaveReview(ctx, runID, review); err != nil {
			warnings = appendWarning(warnings, "persist_review_failed: "+err.Error())
		}
		if err := s.repo.FinalizeRun(ctx, runID, summarizeRunStatus(guardedOrders)); err != nil {
			warnings = appendWarning(warnings, "finalize_run_failed: "+err.Error())
		}
	}

	println("\n" + strings.Repeat("=", 80))
	println("[DECISION SERVICE] 交易决策流程完成")
	println(strings.Repeat("=", 80) + "\n")

	return DecisionResponse{
		Decision:      decision,
		Raw:           resp.Message.Content,
		Signals:       signals,
		Executions:    executions,
		ToolResults:   toolResults,
		RunID:         runIDText,
		RiskBudget:    riskBudget,
		PlannedOrders: plannedOrders,
		GuardedOrders: guardedOrders,
		Review:        review,
		Warnings:      warnings,
	}, nil
}

func (s *Service) finalizeRunWithWarning(ctx context.Context, runID int64, status string, warnings *[]string) error {
	if runID == 0 || s.repo == nil {
		return nil
	}
	err := s.repo.FinalizeRun(ctx, runID, status)
	if err != nil {
		*warnings = appendWarning(*warnings, "finalize_run_failed: "+err.Error())
	}
	return err
}

func (s *Service) resolveRiskBudget(meta map[string]string) RiskBudget {
	budget := s.riskBudgetDefaults.Normalize()
	if meta == nil {
		return budget
	}
	if v, ok := parseMetaFloat(meta, "risk.max_single_order_ratio"); ok {
		budget.MaxSingleOrderRatio = v
	}
	if v, ok := parseMetaFloat(meta, "risk.max_symbol_exposure_ratio"); ok {
		budget.MaxSymbolExposureRatio = v
	}
	if v, ok := parseMetaFloat(meta, "risk.max_daily_trade_ratio"); ok {
		budget.MaxDailyTradeRatio = v
	}
	if v, ok := parseMetaFloat(meta, "risk.min_confidence"); ok {
		budget.MinConfidence = v
	}
	return budget.Normalize()
}

func parseMetaFloat(meta map[string]string, key string) (float64, bool) {
	raw := strings.TrimSpace(meta[key])
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func (s *Service) planExecution(ctx context.Context, accountID int64, decision trade_signal.DecisionOutput, signals []trade_signal.TradeSignal, holdings string, market string, news string, reports []string) ([]PlannedOrder, error) {
	if s.plannerAgent == nil {
		return derivePlannedOrdersFromSignals(signals), nil
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"decision":       decision,
		"signals":        signals,
		"holdings":       holdings,
		"market_summary": market,
		"news_summary":   news,
		"reports":        reports,
	})
	resp, err := s.plannerAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: "将候选交易信号转换为可执行订单，严格输出JSON：\n" + string(payload)}},
	})
	if err != nil {
		return nil, err
	}
	orders, err := parsePlannerOrders(resp.Message.Content, accountID, decision.DecisionID, signals)
	if err != nil {
		return nil, err
	}
	return orders, nil
}

func derivePlannedOrdersFromSignals(signals []trade_signal.TradeSignal) []PlannedOrder {
	orders := make([]PlannedOrder, 0, len(signals))
	seq := 1
	for _, sig := range signals {
		action := strings.ToLower(strings.TrimSpace(sig.Action))
		if action == "hold" {
			continue
		}
		orders = append(orders, PlannedOrder{
			OrderID:        fmt.Sprintf("%s-%d", strings.TrimSpace(sig.DecisionID), seq),
			AccountID:      sig.AccountID,
			Symbol:         strings.TrimSpace(sig.Symbol),
			Name:           strings.TrimSpace(sig.Name),
			AssetType:      strings.TrimSpace(sig.AssetType),
			Action:         action,
			Quantity:       sig.Quantity,
			Price:          sig.Price,
			Notional:       sig.Quantity * sig.Price,
			Confidence:     sig.Confidence,
			DecisionID:     strings.TrimSpace(sig.DecisionID),
			PlanningReason: strings.TrimSpace(sig.Reason),
		})
		seq++
	}
	if orders == nil {
		return []PlannedOrder{}
	}
	return orders
}

func parsePlannerOrders(raw string, accountID int64, decisionID string, signals []trade_signal.TradeSignal) ([]PlannedOrder, error) {
	text := cleanJSONEnvelope(raw)
	if text == "" {
		return nil, errors.New("empty_planner_output")
	}
	var output plannerOutput
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return nil, fmt.Errorf("invalid_planner_json: %w", err)
	}
	if len(output.PlannedOrders) == 0 {
		return []PlannedOrder{}, nil
	}

	templateBySymbolAction := map[string]trade_signal.TradeSignal{}
	for _, sig := range signals {
		key := strings.ToUpper(strings.TrimSpace(sig.Symbol)) + "|" + strings.ToLower(strings.TrimSpace(sig.Action))
		templateBySymbolAction[key] = sig
	}

	orders := make([]PlannedOrder, 0, len(output.PlannedOrders))
	for i, item := range output.PlannedOrders {
		action := strings.ToLower(strings.TrimSpace(item.Action))
		if action == "hold" {
			continue
		}
		if action != "buy" && action != "sell" {
			return nil, fmt.Errorf("invalid_order_action: index=%d action=%s", i, action)
		}
		if item.Quantity <= 0 || item.Price <= 0 {
			return nil, fmt.Errorf("invalid_order_value: index=%d", i)
		}
		orderID := strings.TrimSpace(item.OrderID)
		if orderID == "" {
			orderID = fmt.Sprintf("%s-%d", decisionID, i+1)
		}
		symbol := strings.TrimSpace(item.Symbol)
		if symbol == "" {
			return nil, fmt.Errorf("missing_order_symbol: index=%d", i)
		}

		tpl := templateBySymbolAction[strings.ToUpper(symbol)+"|"+action]
		ord := PlannedOrder{
			OrderID:        orderID,
			AccountID:      item.AccountID,
			Symbol:         symbol,
			Name:           strings.TrimSpace(item.Name),
			AssetType:      strings.TrimSpace(item.AssetType),
			Action:         action,
			Quantity:       item.Quantity,
			Price:          item.Price,
			Notional:       item.Quantity * item.Price,
			Confidence:     item.Confidence,
			DecisionID:     decisionID,
			PlanningReason: strings.TrimSpace(item.PlanningReason),
		}
		if ord.AccountID == 0 {
			ord.AccountID = accountID
		}
		if ord.AccountID == 0 {
			ord.AccountID = tpl.AccountID
		}
		if ord.Name == "" {
			ord.Name = strings.TrimSpace(tpl.Name)
		}
		if ord.AssetType == "" {
			ord.AssetType = strings.TrimSpace(tpl.AssetType)
		}
		if ord.Confidence <= 0 {
			ord.Confidence = tpl.Confidence
		}
		if ord.PlanningReason == "" {
			ord.PlanningReason = strings.TrimSpace(tpl.Reason)
		}
		orders = append(orders, ord)
	}
	if orders == nil {
		return []PlannedOrder{}, nil
	}
	return orders, nil
}

func (s *Service) guardOrders(ctx context.Context, accountID int64, budget RiskBudget, planned []PlannedOrder) ([]GuardedOrder, error) {
	if len(planned) == 0 {
		return []GuardedOrder{}, nil
	}
	if s.policyGuard == nil {
		fallback := make([]GuardedOrder, 0, len(planned))
		for _, order := range planned {
			fallback = append(fallback, GuardedOrder{
				PlannedOrder:    order,
				GuardStatus:     "approved",
				ExecutionStatus: "pending",
			})
		}
		return fallback, nil
	}
	return s.policyGuard.Guard(ctx, GuardRequest{AccountID: accountID, RiskBudget: budget, PlannedOrders: planned})
}

func (s *Service) executeGuardedOrders(ctx context.Context, decisionID string, guarded []GuardedOrder) ([]trade_signal.ExecutionResult, []GuardedOrder, error) {
	if len(guarded) == 0 {
		return []trade_signal.ExecutionResult{}, guarded, nil
	}
	approvedSignals := make([]trade_signal.TradeSignal, 0, len(guarded))
	approvedIndexes := make([]int, 0, len(guarded))
	execByIndex := map[int]trade_signal.ExecutionResult{}

	for i := range guarded {
		item := &guarded[i]
		signal := plannedOrderToSignal(item.PlannedOrder, decisionID)
		if item.GuardStatus != "approved" {
			if item.ExecutionStatus == "" {
				item.ExecutionStatus = "blocked"
			}
			if item.ExecutionError == "" {
				item.ExecutionError = item.GuardReason
			}
			execByIndex[i] = trade_signal.ExecutionResult{
				Signal: signal,
				Status: "blocked",
				Error:  item.GuardReason,
			}
			continue
		}
		approvedIndexes = append(approvedIndexes, i)
		approvedSignals = append(approvedSignals, signal)
	}

	if len(approvedSignals) > 0 {
		if s.engine == nil {
			for i, signal := range approvedSignals {
				idx := approvedIndexes[i]
				guarded[idx].ExecutionStatus = "skipped"
				execByIndex[idx] = trade_signal.ExecutionResult{Signal: signal, Status: "skipped"}
			}
		} else {
			results, err := s.engine.Execute(ctx, approvedSignals)
			if err != nil {
				return nil, guarded, err
			}
			for i, idx := range approvedIndexes {
				if i >= len(results) {
					guarded[idx].ExecutionStatus = "failed"
					guarded[idx].ExecutionError = "missing_execution_result"
					execByIndex[idx] = trade_signal.ExecutionResult{
						Signal: approvedSignals[i],
						Status: "failed",
						Error:  "missing_execution_result",
					}
					continue
				}
				res := results[i]
				guarded[idx].ExecutionStatus = strings.TrimSpace(res.Status)
				guarded[idx].ExecutionError = strings.TrimSpace(res.Error)
				if res.Trade != nil {
					guarded[idx].TradeID = res.Trade.ID
				}
				execByIndex[idx] = res
			}
		}
	}

	executions := make([]trade_signal.ExecutionResult, 0, len(guarded))
	for i := range guarded {
		res, ok := execByIndex[i]
		if !ok {
			res = trade_signal.ExecutionResult{Signal: plannedOrderToSignal(guarded[i].PlannedOrder, decisionID), Status: "unknown"}
		}
		executions = append(executions, res)
	}
	return executions, guarded, nil
}

func plannedOrderToSignal(order PlannedOrder, decisionID string) trade_signal.TradeSignal {
	return trade_signal.TradeSignal{
		AccountID:  order.AccountID,
		Symbol:     order.Symbol,
		Name:       order.Name,
		AssetType:  order.AssetType,
		Action:     order.Action,
		Quantity:   order.Quantity,
		Price:      order.Price,
		Confidence: order.Confidence,
		Reason:     order.PlanningReason,
		DecisionID: decisionID,
	}
}

func (s *Service) generateReview(ctx context.Context, budget RiskBudget, planned []PlannedOrder, guarded []GuardedOrder, executions []trade_signal.ExecutionResult, toolResults []tool.ToolResult) (*PostTradeReview, error) {
	if s.reviewAgent == nil {
		return nil, nil
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"risk_budget":    budget,
		"planned_orders": planned,
		"guarded_orders": guarded,
		"executions":     executions,
		"tool_results":   toolResults,
	})
	resp, err := s.reviewAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: "基于执行结果生成复盘归因，严格输出JSON：\n" + string(payload)}},
	})
	if err != nil {
		return nil, err
	}
	return parseReview(resp.Message.Content)
}

func parseReview(raw string) (*PostTradeReview, error) {
	text := cleanJSONEnvelope(raw)
	if text == "" {
		return nil, errors.New("empty_review_output")
	}
	var out reviewOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("invalid_review_json: %w", err)
	}
	if strings.TrimSpace(out.Summary) == "" {
		return nil, errors.New("empty_review_summary")
	}
	return &PostTradeReview{
		Summary:        strings.TrimSpace(out.Summary),
		Attributions:   out.Attributions,
		LearningPoints: out.LearningPoints,
	}, nil
}

func cleanJSONEnvelope(raw string) string {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}

func summarizeRunStatus(orders []GuardedOrder) string {
	if len(orders) == 0 {
		return "completed"
	}
	blocked := 0
	failed := 0
	for _, order := range orders {
		if order.GuardStatus == "blocked" {
			blocked++
		}
		if order.ExecutionStatus == "failed" {
			failed++
		}
	}
	if blocked == len(orders) {
		return "blocked"
	}
	if blocked > 0 || failed > 0 {
		return "partial"
	}
	return "completed"
}

func parseToolResultsMeta(meta map[string]string) []tool.ToolResult {
	if meta == nil {
		return nil
	}
	raw := strings.TrimSpace(meta["tool_results"])
	if raw == "" {
		return nil
	}
	results := []tool.ToolResult{}
	if err := json.Unmarshal([]byte(raw), &results); err != nil {
		return nil
	}
	return results
}

func buildDecisionInput(req DecisionRequest, holdings string, market string, news string, reports []string, selectedGuides []decisionGuideInput) string {
	payloadMap := map[string]interface{}{
		"holdings":       holdings,
		"market_summary": market,
		"news_summary":   news,
		"reports":        reports,
		"meta":           req.Meta,
	}
	if strings.TrimSpace(news) == "" {
		delete(payloadMap, "news_summary")
	}
	if len(selectedGuides) > 0 {
		payloadMap["selected_trade_guides"] = selectedGuides
	}
	payload, _ := json.Marshal(payloadMap)
	return "生成交易决策，严格输出JSON：\n" + strings.TrimSpace(string(payload))
}

func (s *Service) loadReports(ctx context.Context, ids []int64) ([]string, error) {
	if s.reports == nil || len(ids) == 0 {
		return nil, nil
	}
	results := make([]string, 0, len(ids))
	for _, id := range ids {
		report, err := s.reports.GetReport(ctx, id)
		if err != nil {
			return nil, err
		}
		results = append(results, report.Summary)
	}
	return results, nil
}

func (s *Service) loadHoldings(ctx context.Context, accountID int64) (string, error) {
	if s.holdings == nil || accountID == 0 {
		return "", nil
	}
	positions, err := s.holdings.ListPositions(ctx, accountID)
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(positions)
	return string(payload), nil
}

func (s *Service) loadMarketNews(ctx context.Context) (string, string) {
	if s.provider == nil {
		return "", ""
	}
	market, err := s.provider.GetMarketSummary(ctx)
	if err != nil {
		market = ""
	}
	news, err := s.provider.GetNewsSummary(ctx)
	if err != nil {
		news = ""
	}
	return market, news
}

func (s *Service) resolveGuideSelections(ctx context.Context, selections []GuideSelection) ([]decisionGuideInput, error) {
	if len(selections) == 0 {
		return nil, nil
	}
	if s.guides == nil {
		return nil, errors.New("guide_repository_not_initialized")
	}
	seen := map[string]int{}
	out := make([]decisionGuideInput, 0, len(selections))
	for _, selection := range selections {
		symbol := strings.TrimSpace(selection.Symbol)
		if symbol == "" || selection.GuideID <= 0 {
			continue
		}
		guide, err := s.guides.GetGuideByID(ctx, selection.GuideID)
		if err != nil {
			return nil, err
		}
		if guide == nil {
			return nil, fmt.Errorf("guide_not_found: %d", selection.GuideID)
		}
		if !strings.EqualFold(strings.TrimSpace(guide.Symbol), symbol) {
			return nil, fmt.Errorf("guide_symbol_mismatch: guide_id=%d symbol=%s guide_symbol=%s", selection.GuideID, symbol, guide.Symbol)
		}
		item := decisionGuideInput{
			Symbol:            strings.TrimSpace(guide.Symbol),
			GuideID:           selection.GuideID,
			TradeGuideText:    strings.TrimSpace(guide.TradeGuideText),
			TradeGuideJSON:    strings.TrimSpace(guide.TradeGuideJSON),
			TradeGuideVersion: strings.TrimSpace(guide.TradeGuideVersion),
			BuyConditions:     guide.BuyConditions,
			SellConditions:    guide.SellConditions,
			StopLoss:          strings.TrimSpace(guide.StopLoss),
			TakeProfit:        strings.TrimSpace(guide.TakeProfit),
			RiskMonitors:      guide.RiskMonitors,
		}
		if guide.ValidUntil != nil && !guide.ValidUntil.IsZero() {
			item.ValidUntil = guide.ValidUntil.Format("2006-01-02")
		}
		key := strings.ToUpper(item.Symbol)
		if idx, exists := seen[key]; exists {
			out[idx] = item
			continue
		}
		seen[key] = len(out)
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) persistGuideSelections(ctx context.Context, accountID int64, guides []decisionGuideInput) error {
	if accountID == 0 || s.holdings == nil || len(guides) == 0 {
		return nil
	}
	for _, guide := range guides {
		if strings.TrimSpace(guide.Symbol) == "" || guide.GuideID <= 0 {
			continue
		}
		if err := s.holdings.SetPositionOperationGuideBySymbol(ctx, accountID, guide.Symbol, guide.GuideID); err != nil {
			return err
		}
	}
	return nil
}

func appendWarning(existing []string, warning string) []string {
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return existing
	}
	for _, item := range existing {
		if item == warning {
			return existing
		}
	}
	return append(existing, warning)
}

// 辅助函数
func printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
