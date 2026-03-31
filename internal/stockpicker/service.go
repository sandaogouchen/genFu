package stockpicker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"genFu/internal/agent"
	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/tool"
)

// Service 选股服务
type Service struct {
	regimeAgent             agent.Agent // 市场状态识别Agent
	screenerAgent           agent.Agent // 筛选Agent：生成筛选策略
	analyzerAgent           agent.Agent // 分析Agent：深度分析筛选结果
	portfolioFitAgent       agent.Agent // 组合约束匹配Agent
	tradeGuideCompilerAgent agent.Agent // 交易指南编译Agent
	registry                *tool.Registry
	provider                DataProvider
	allocationService       *AllocationService
	guideRepo               *GuideRepository
	runRepo                 *RunRepository
}

// NewService 创建选股服务
func NewService(
	regimeAgent agent.Agent,
	screenerAgent agent.Agent,
	analyzerAgent agent.Agent,
	portfolioFitAgent agent.Agent,
	tradeGuideCompilerAgent agent.Agent,
	registry *tool.Registry,
	provider DataProvider,
	guideRepo *GuideRepository,
	runRepo *RunRepository,
) *Service {
	return &Service{
		regimeAgent:             regimeAgent,
		screenerAgent:           screenerAgent,
		analyzerAgent:           analyzerAgent,
		portfolioFitAgent:       portfolioFitAgent,
		tradeGuideCompilerAgent: tradeGuideCompilerAgent,
		registry:                registry,
		provider:                provider,
		allocationService:       NewAllocationService(),
		guideRepo:               guideRepo,
		runRepo:                 runRepo,
	}
}

// PickStocks 执行选股
func (s *Service) PickStocks(ctx context.Context, req StockPickRequest) (resp StockPickResponse, retErr error) {
	totalStartTime := time.Now()
	if s == nil {
		return StockPickResponse{}, errors.New("service_not_initialized")
	}

	if req.TopN <= 0 {
		req.TopN = 5
	}
	if req.DateTo.IsZero() {
		req.DateTo = time.Now()
	}
	if req.DateFrom.IsZero() {
		req.DateFrom = req.DateTo.AddDate(0, 0, -3)
	}
	req.RiskProfile = normalizeRiskProfile(req.RiskProfile)

	days := int(req.DateTo.Sub(req.DateFrom).Hours()/24) + 1
	if days <= 0 {
		days = 3
	}

	pickID := fmt.Sprintf("pick_%d", time.Now().UnixNano())
	warnings := make([]string, 0, 8)
	var marketData MarketData
	var newsEvents []NewsEvent
	var holdings []Position
	var stockList []tool.MarketItem
	var regimeOutput *RegimeOutput
	var screeningOutput *AgentScreeningOutput
	var screeningResult *ScreeningResult
	var analysisSnapshot interface{}
	var routingSnapshot interface{}
	var candidateSnapshot interface{}
	var portfolioFitSnapshot interface{}
	var tradeGuidesSnapshot interface{}

	defer func() {
		if s.runRepo == nil {
			return
		}
		status := "completed"
		errorMessage := ""
		if retErr != nil {
			status = "failed"
			errorMessage = retErr.Error()
		}
		saveErr := s.runRepo.SaveByPickID(ctx, pickID, StockPickRunSnapshot{
			Request:       req,
			MarketData:    marketData,
			Regime:        regimeOutput,
			Routing:       routingSnapshot,
			CandidatePool: candidateSnapshot,
			Analysis:      analysisSnapshot,
			PortfolioFit:  portfolioFitSnapshot,
			TradeGuides:   tradeGuidesSnapshot,
			Warnings:      warnings,
			Status:        status,
			ErrorMessage:  errorMessage,
		})
		if saveErr != nil {
			log.Printf("[选股服务] 保存运行快照失败 pick_id=%s err=%v", pickID, saveErr)
		}
	}()

	log.Printf("[选股服务] 开始执行选股 topN=%d days=%d risk_profile=%s", req.TopN, days, req.RiskProfile)

	// 1) 准备数据
	dataStartTime := time.Now()
	var err error
	marketData, err = s.provider.GetMarketData(ctx, days)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("get_market_data_failed: %w", err)
	}
	newsEvents, err = s.provider.GetRecentNews(ctx, days, 50)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("get_news_failed: %w", err)
	}
	if req.AccountID > 0 {
		holdings, err = s.provider.GetHoldings(ctx, req.AccountID)
		if err != nil {
			holdings = []Position{}
		}
	}
	stockList, err = s.provider.GetStockList(ctx)
	if err != nil {
		log.Printf("[选股服务] 获取股票列表失败，降级继续 err=%v", err)
		stockList = []tool.MarketItem{}
	}
	warnings = append(warnings, s.buildWarnings(marketData, newsEvents, stockList)...)
	log.Printf("[选股服务] 数据准备完成 耗时=%v 新闻数=%d 股票数=%d", time.Since(dataStartTime), len(newsEvents), len(stockList))

	// 2) 市场状态识别
	regimeOutput, err = s.runRegimeAgent(ctx, req, marketData, newsEvents, holdings)
	if err != nil {
		warnings = append(warnings, "RegimeAgent失败，已降级为启发式市场状态识别")
		fallback := fallbackRegimeFromMarketData(marketData)
		regimeOutput = &fallback
	}

	// 3) 策略路由 + 候选池
	screenerInput := s.buildScreenerInput(req, marketData, newsEvents, holdings, regimeOutput)
	log.Printf("[选股服务] 调用筛选Agent...")
	screenerResp, err := s.screenerAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: screenerInput}},
	})
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("screener_agent_failed: %w", err)
	}
	screeningOutput, err = s.parseScreenerOutput(screenerResp.Message.Content)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("parse_screener_output_failed: %w", err)
	}
	routingSnapshot = map[string]interface{}{
		"strategy_name":        screeningOutput.StrategyName,
		"strategy_description": screeningOutput.StrategyDescription,
		"screening_conditions": screeningOutput.ScreeningConditions,
		"market_context":       screeningOutput.MarketContext,
		"risk_notes":           screeningOutput.RiskNotes,
		"market_regime":        regimeOutput.MarketRegime,
	}

	screeningResult, err = s.executeScreening(ctx, screeningOutput.ScreeningConditions)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("execute_screening_failed: %w", err)
	}
	candidateSnapshot = screeningResult

	// 4) 深度分析
	analyzerInput := s.buildAnalyzerInput(req, marketData, newsEvents, holdings, screeningResult, screeningOutput, regimeOutput)
	log.Printf("[选股服务] 调用分析Agent...")
	analyzerResp, err := s.analyzerAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: analyzerInput}},
	})
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("analyzer_agent_failed: %w", err)
	}
	output, err := s.parseAgentOutput(analyzerResp.Message.Content)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("parse_output_failed: %w", err)
	}
	analysisSnapshot = cloneJSON(output)

	for i := range output.Stocks {
		allocation := s.allocationService.CalculateAllocation(
			&output.Stocks[i],
			holdings,
			stockList,
		)
		output.Stocks[i].Allocation = allocation
	}

	originalStocks := append([]StockPick(nil), output.Stocks...)

	// 5) 组合约束重排（先硬过滤，再软排序）
	portfolioFitOutput, fitErr := s.runPortfolioFitAgent(ctx, req, regimeOutput, output.Stocks, holdings, screeningResult)
	if fitErr != nil {
		warnings = append(warnings, "PortfolioFitAgent失败，已降级为基础权重模型")
		s.applyFallbackPortfolioFit(output.Stocks, req.RiskProfile)
		portfolioFitSnapshot = map[string]interface{}{"fallback": "allocation_only", "error": fitErr.Error()}
	} else {
		output.Stocks = s.applyPortfolioFit(output.Stocks, portfolioFitOutput, req.RiskProfile)
		portfolioFitSnapshot = portfolioFitOutput
		if len(output.Stocks) == 0 {
			warnings = append(warnings, "PortfolioFitAgent硬约束后无可用标的，已回退分析结果")
			output.Stocks = originalStocks
			s.applyFallbackPortfolioFit(output.Stocks, req.RiskProfile)
		}
	}

	// 6) 交易指南编译（先保留确定性v1兜底，再尝试编译v2）
	s.attachTradeGuides(output, screeningOutput, screeningResult)
	compilerOutput, compileErr := s.runTradeGuideCompilerAgent(ctx, req, regimeOutput, screeningOutput, screeningResult, output.Stocks)
	if compileErr != nil {
		warnings = append(warnings, "TradeGuideCompilerAgent失败，已降级使用确定性交易规则")
		s.fillTradeGuideV2Fallback(output)
	} else {
		s.applyCompiledTradeGuides(output, compilerOutput)
	}

	if len(output.Stocks) > req.TopN {
		output.Stocks = output.Stocks[:req.TopN]
	}

	// 持久化指南
	for i := range output.Stocks {
		if s.guideRepo == nil {
			continue
		}
		guide := buildPersistableGuide(&output.Stocks[i], pickID)
		if guide == nil {
			continue
		}
		validUntil := time.Now().AddDate(0, 0, 30)
		guide.ValidUntil = &validUntil
		if err := s.guideRepo.SaveGuide(ctx, guide); err != nil {
			log.Printf("[选股服务] save operation guide failed: %v", err)
		}
	}

	tradeGuidesSnapshot = cloneJSON(output.Stocks)

	log.Printf("[选股服务] 选股完成 最终股票数=%d 总耗时=%v", len(output.Stocks), time.Since(totalStartTime))
	resp = StockPickResponse{
		PickID:           pickID,
		GeneratedAt:      time.Now(),
		Stocks:           output.Stocks,
		MarketData:       marketData,
		NewsSummary:      output.MarketView,
		MarketRegime:     regimeOutput.MarketRegime,
		RegimeConfidence: regimeOutput.RegimeConfidence,
		RegimeReasoning:  regimeOutput.RegimeReasoning,
		Warnings:         warnings,
		ScreeningInfo:    screeningResult,
	}
	return resp, nil
}

// buildScreenerInput 构建筛选Agent输入
func (s *Service) buildScreenerInput(
	req StockPickRequest,
	marketData MarketData,
	newsEvents []NewsEvent,
	holdings []Position,
	regime *RegimeOutput,
) string {
	payload := map[string]interface{}{
		"request":     req,
		"market_data": marketData,
		"news_events": newsEvents,
		"holdings":    holdings,
		"regime":      regime,
	}
	raw, _ := json.Marshal(payload)
	return fmt.Sprintf("请根据当前市场状况，生成股票筛选策略，严格输出JSON：\n%s", string(raw))
}

// parseScreenerOutput 解析筛选Agent输出
func (s *Service) parseScreenerOutput(content string) (*AgentScreeningOutput, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var output AgentScreeningOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return nil, fmt.Errorf("json_parse_error: %w", err)
	}
	return &output, nil
}

// executeScreening 执行股票筛选
func (s *Service) executeScreening(ctx context.Context, conditions ScreeningRequest) (*ScreeningResult, error) {
	args := map[string]interface{}{
		"action": "screen",
	}

	if conditions.StrategyType != "" {
		args["strategy_type"] = conditions.StrategyType
	}
	if conditions.PriceMin != nil {
		args["price_min"] = *conditions.PriceMin
	}
	if conditions.PriceMax != nil {
		args["price_max"] = *conditions.PriceMax
	}
	if conditions.ChangeRateMin != nil {
		args["change_rate_min"] = *conditions.ChangeRateMin
	}
	if conditions.ChangeRateMax != nil {
		args["change_rate_max"] = *conditions.ChangeRateMax
	}
	if conditions.AmountMin != nil {
		args["amount_min"] = *conditions.AmountMin
	}
	if conditions.AmountMax != nil {
		args["amount_max"] = *conditions.AmountMax
	}
	if conditions.MA5AboveMA20 != nil {
		args["ma5_above_ma20"] = *conditions.MA5AboveMA20
	}
	if conditions.MA20Rising != nil {
		args["ma20_rising"] = *conditions.MA20Rising
	}
	if conditions.MACDGolden != nil {
		args["macd_golden"] = *conditions.MACDGolden
	}
	if conditions.RSIOversold != nil {
		args["rsi_oversold"] = *conditions.RSIOversold
	}
	if conditions.RSIOverbought != nil {
		args["rsi_overbought"] = *conditions.RSIOverbought
	}
	if conditions.VolumeSpike != nil {
		args["volume_spike"] = *conditions.VolumeSpike
	}
	if conditions.Limit > 0 {
		args["limit"] = conditions.Limit
	}

	result, err := s.registry.Execute(ctx, tool.ToolCall{
		Name:      "stock_screener",
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	var screeningResult ScreeningResult
	data, _ := json.Marshal(result.Output)
	if err := json.Unmarshal(data, &screeningResult); err != nil {
		return nil, err
	}
	return &screeningResult, nil
}

// buildAnalyzerInput 构建分析Agent输入
func (s *Service) buildAnalyzerInput(
	req StockPickRequest,
	marketData MarketData,
	newsEvents []NewsEvent,
	holdings []Position,
	screeningResult *ScreeningResult,
	screeningOutput *AgentScreeningOutput,
	regime *RegimeOutput,
) string {
	payload := map[string]interface{}{
		"request":            req,
		"market_data":        marketData,
		"news_events":        newsEvents,
		"holdings":           holdings,
		"screening_result":   screeningResult,
		"screening_strategy": screeningOutput,
		"regime":             regime,
	}
	raw, _ := json.Marshal(payload)
	return fmt.Sprintf("请对以下筛选后的候选股票进行深度分析和验证，严格输出JSON：\n%s", string(raw))
}

// buildAgentInput 构建Agent输入
func (s *Service) buildAgentInput(
	req StockPickRequest,
	marketData MarketData,
	newsEvents []NewsEvent,
	holdings []Position,
	stockList []tool.MarketItem,
) string {
	payload := map[string]interface{}{
		"request":     req,
		"market_data": marketData,
		"news_events": newsEvents,
		"holdings":    holdings,
		"stock_count": len(stockList),
	}

	raw, _ := json.Marshal(payload)
	return fmt.Sprintf("请根据以下数据生成选股建议，严格输出JSON：\n%s", string(raw))
}

// parseAgentOutput 解析Agent输出
func (s *Service) parseAgentOutput(content string) (*AgentOutput, error) {
	// 清理可能的Markdown标记
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil, fmt.Errorf("json_parse_error: %w", err)
	}

	var output AgentOutput
	if raw, ok := root["stocks"]; ok {
		if err := json.Unmarshal(raw, &output.Stocks); err != nil {
			return nil, fmt.Errorf("stocks_parse_error: %w", err)
		}
	}
	if raw, ok := root["market_view"]; ok {
		_ = json.Unmarshal(raw, &output.MarketView)
	}
	if raw, ok := root["risk_notes"]; ok {
		_ = json.Unmarshal(raw, &output.RiskNotes)
	}

	return &output, nil
}

func normalizeRiskProfile(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "conservative":
		return "conservative"
	case "aggressive":
		return "aggressive"
	default:
		return "balanced"
	}
}

func riskBudgetCapForProfile(profile string) float64 {
	switch normalizeRiskProfile(profile) {
	case "conservative":
		return 0.15
	case "aggressive":
		return 0.30
	default:
		return 0.20
	}
}

func (s *Service) runRegimeAgent(
	ctx context.Context,
	req StockPickRequest,
	marketData MarketData,
	newsEvents []NewsEvent,
	holdings []Position,
) (*RegimeOutput, error) {
	if s.regimeAgent == nil {
		return nil, errors.New("regime_agent_not_initialized")
	}
	payload := map[string]interface{}{
		"request":     req,
		"market_data": marketData,
		"news_events": newsEvents,
		"holdings":    holdings,
	}
	raw, _ := json.Marshal(payload)
	resp, err := s.regimeAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: fmt.Sprintf("识别当前市场状态，严格输出JSON：\n%s", string(raw))}},
	})
	if err != nil {
		return nil, err
	}
	return parseRegimeOutput(resp.Message.Content)
}

func parseRegimeOutput(content string) (*RegimeOutput, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var output RegimeOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return nil, fmt.Errorf("regime_json_parse_error: %w", err)
	}
	output.MarketRegime = strings.TrimSpace(output.MarketRegime)
	if output.MarketRegime == "" {
		return nil, errors.New("empty_market_regime")
	}
	output.RegimeConfidence = clampUnit(output.RegimeConfidence)
	return &output, nil
}

func fallbackRegimeFromMarketData(marketData MarketData) RegimeOutput {
	total := marketData.UpCount + marketData.DownCount
	upRatio := 0.5
	if total > 0 {
		upRatio = float64(marketData.UpCount) / float64(total)
	}
	switch {
	case upRatio >= 0.62:
		return RegimeOutput{MarketRegime: "trend_up", RegimeConfidence: 0.62, RegimeReasoning: "上涨家数占优，市场偏强"}
	case upRatio <= 0.38:
		return RegimeOutput{MarketRegime: "trend_down", RegimeConfidence: 0.62, RegimeReasoning: "下跌家数占优，市场偏弱"}
	case upRatio > 0.55:
		return RegimeOutput{MarketRegime: "risk_on", RegimeConfidence: 0.55, RegimeReasoning: "风险偏好回升"}
	case upRatio < 0.45:
		return RegimeOutput{MarketRegime: "risk_off", RegimeConfidence: 0.55, RegimeReasoning: "风险偏好下降"}
	default:
		return RegimeOutput{MarketRegime: "range", RegimeConfidence: 0.5, RegimeReasoning: "多空均衡，震荡市"}
	}
}

func (s *Service) runPortfolioFitAgent(
	ctx context.Context,
	req StockPickRequest,
	regime *RegimeOutput,
	stocks []StockPick,
	holdings []Position,
	screening *ScreeningResult,
) (*PortfolioFitOutput, error) {
	if s.portfolioFitAgent == nil {
		return nil, errors.New("portfolio_fit_agent_not_initialized")
	}
	payload := map[string]interface{}{
		"risk_profile":   req.RiskProfile,
		"market_regime":  regime,
		"holdings":       holdings,
		"candidates":     stocks,
		"screening_info": screening,
	}
	raw, _ := json.Marshal(payload)
	resp, err := s.portfolioFitAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: fmt.Sprintf("请输出组合约束重排结果，严格JSON：\n%s", string(raw))}},
	})
	if err != nil {
		return nil, err
	}
	return parsePortfolioFitOutput(resp.Message.Content)
}

func parsePortfolioFitOutput(content string) (*PortfolioFitOutput, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var output PortfolioFitOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return nil, fmt.Errorf("portfolio_fit_json_parse_error: %w", err)
	}
	if len(output.Stocks) == 0 {
		return nil, errors.New("empty_portfolio_fit_stocks")
	}
	for i := range output.Stocks {
		output.Stocks[i].Symbol = strings.TrimSpace(output.Stocks[i].Symbol)
		output.Stocks[i].FitScore = clampUnit(output.Stocks[i].FitScore)
		output.Stocks[i].RiskBudgetWeight = clampUnit(output.Stocks[i].RiskBudgetWeight)
	}
	return &output, nil
}

func (s *Service) applyPortfolioFit(stocks []StockPick, fit *PortfolioFitOutput, riskProfile string) []StockPick {
	if len(stocks) == 0 || fit == nil {
		return stocks
	}
	bySymbol := make(map[string]PortfolioFitRecord, len(fit.Stocks))
	for _, item := range fit.Stocks {
		symbol := strings.TrimSpace(item.Symbol)
		if symbol == "" {
			continue
		}
		bySymbol[symbol] = item
	}

	cap := riskBudgetCapForProfile(riskProfile)
	filtered := make([]StockPick, 0, len(stocks))
	for i := range stocks {
		item, ok := bySymbol[stocks[i].Symbol]
		if !ok {
			continue
		}
		if item.HardReject {
			continue
		}
		if item.RiskBudgetWeight > cap {
			continue
		}
		stocks[i].FitScore = item.FitScore
		stocks[i].RiskBudgetWeight = item.RiskBudgetWeight
		stocks[i].FitReasons = append([]string{}, item.FitReasons...)
		if stocks[i].Allocation.SuggestedWeight > item.RiskBudgetWeight && item.RiskBudgetWeight > 0 {
			stocks[i].Allocation.SuggestedWeight = item.RiskBudgetWeight
		}
		filtered = append(filtered, stocks[i])
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].FitScore == filtered[j].FitScore {
			return filtered[i].RiskBudgetWeight > filtered[j].RiskBudgetWeight
		}
		return filtered[i].FitScore > filtered[j].FitScore
	})
	return filtered
}

func (s *Service) applyFallbackPortfolioFit(stocks []StockPick, riskProfile string) {
	cap := riskBudgetCapForProfile(riskProfile)
	for i := range stocks {
		weight := stocks[i].Allocation.SuggestedWeight
		if weight <= 0 {
			weight = 0.1
		}
		if weight > cap {
			weight = cap
		}
		stocks[i].FitScore = clampUnit(0.5 + stocks[i].Confidence*0.4)
		stocks[i].RiskBudgetWeight = weight
		if len(stocks[i].FitReasons) == 0 {
			stocks[i].FitReasons = []string{"使用基础仓位模型降级估计"}
		}
	}
}

func (s *Service) runTradeGuideCompilerAgent(
	ctx context.Context,
	req StockPickRequest,
	regime *RegimeOutput,
	screeningOutput *AgentScreeningOutput,
	screeningResult *ScreeningResult,
	stocks []StockPick,
) (*TradeGuideCompilerOutput, error) {
	if s.tradeGuideCompilerAgent == nil {
		return nil, errors.New("trade_guide_compiler_agent_not_initialized")
	}
	payload := map[string]interface{}{
		"request":            req,
		"regime":             regime,
		"screening_strategy": screeningOutput,
		"screening_result":   screeningResult,
		"stocks":             stocks,
	}
	raw, _ := json.Marshal(payload)
	resp, err := s.tradeGuideCompilerAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: fmt.Sprintf("编译交易规则并输出JSON：\n%s", string(raw))}},
	})
	if err != nil {
		return nil, err
	}
	return parseTradeGuideCompilerOutput(resp.Message.Content)
}

func parseTradeGuideCompilerOutput(content string) (*TradeGuideCompilerOutput, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var output TradeGuideCompilerOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return nil, fmt.Errorf("trade_guide_compiler_json_parse_error: %w", err)
	}
	if len(output.Stocks) == 0 {
		return nil, errors.New("empty_trade_guide_compiler_stocks")
	}
	return &output, nil
}

func (s *Service) applyCompiledTradeGuides(output *AgentOutput, compiled *TradeGuideCompilerOutput) {
	if output == nil || compiled == nil {
		return
	}
	bySymbol := make(map[string]TradeGuideCompilerRecord, len(compiled.Stocks))
	for _, item := range compiled.Stocks {
		symbol := strings.TrimSpace(item.Symbol)
		if symbol == "" {
			continue
		}
		bySymbol[symbol] = item
	}
	for i := range output.Stocks {
		item, ok := bySymbol[output.Stocks[i].Symbol]
		if !ok {
			if strings.TrimSpace(output.Stocks[i].TradeGuideJSONV2) == "" {
				output.Stocks[i].TradeGuideJSONV2 = convertLegacyGuideToV2(output.Stocks[i].TradeGuideJSON)
			}
			continue
		}
		if text := strings.TrimSpace(item.TradeGuideText); text != "" {
			output.Stocks[i].TradeGuideText = text
		}
		v2 := strings.TrimSpace(item.TradeGuideJSONV2)
		if !json.Valid([]byte(v2)) {
			v2 = convertLegacyGuideToV2(output.Stocks[i].TradeGuideJSON)
		}
		output.Stocks[i].TradeGuideJSONV2 = v2

		v1 := strings.TrimSpace(item.TradeGuideJSON)
		if !json.Valid([]byte(v1)) {
			v1 = projectGuideV2ToV1(v2, output.Stocks[i].Symbol)
		}
		if strings.TrimSpace(v1) == "" || !json.Valid([]byte(v1)) {
			v1 = output.Stocks[i].TradeGuideJSON
		}
		output.Stocks[i].TradeGuideJSON = v1
		output.Stocks[i].TradeGuideVersion = "v2"
	}
}

func (s *Service) fillTradeGuideV2Fallback(output *AgentOutput) {
	if output == nil {
		return
	}
	for i := range output.Stocks {
		if strings.TrimSpace(output.Stocks[i].TradeGuideJSONV2) == "" {
			output.Stocks[i].TradeGuideJSONV2 = convertLegacyGuideToV2(output.Stocks[i].TradeGuideJSON)
		}
		if strings.TrimSpace(output.Stocks[i].TradeGuideVersion) == "" {
			output.Stocks[i].TradeGuideVersion = "v1"
		}
	}
}

func convertLegacyGuideToV2(v1 string) string {
	v1 = strings.TrimSpace(v1)
	if v1 == "" || !json.Valid([]byte(v1)) {
		return `{}`
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(v1), &payload); err != nil {
		return `{}`
	}
	out := map[string]interface{}{
		"schema_version": "v2",
		"asset_type":     payload["asset_type"],
		"symbol":         payload["symbol"],
		"entries":        payload["buy_rules"],
		"exits":          payload["sell_rules"],
		"risk_controls":  payload["risk_controls"],
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return `{}`
	}
	return string(raw)
}

func projectGuideV2ToV1(v2 string, symbol string) string {
	v2 = strings.TrimSpace(v2)
	if v2 == "" || !json.Valid([]byte(v2)) {
		return `{}`
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(v2), &payload); err != nil {
		return `{}`
	}
	out := map[string]interface{}{
		"asset_type":    "stock",
		"strategy_type": "compiled_v2",
		"strategy_name": "compiled_v2",
		"symbol":        symbol,
		"buy_rules":     payload["entries"],
		"sell_rules":    payload["exits"],
		"risk_controls": payload["risk_controls"],
	}
	if st, ok := payload["asset_type"]; ok {
		out["asset_type"] = st
	}
	if sym, ok := payload["symbol"]; ok {
		out["symbol"] = sym
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return `{}`
	}
	return string(raw)
}

func cloneJSON(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

type quantitativeRule struct {
	RuleID       string  `json:"rule_id"`
	Indicator    string  `json:"indicator"`
	Operator     string  `json:"operator"`
	TriggerValue float64 `json:"trigger_value"`
	Timeframe    string  `json:"timeframe"`
	Weight       float64 `json:"weight"`
	Note         string  `json:"note"`
}

type quantitativeRiskControls struct {
	StopLossPrice    float64 `json:"stop_loss_price"`
	TakeProfitPrice  float64 `json:"take_profit_price"`
	MaxPositionRatio float64 `json:"max_position_ratio"`
}

type stockTradeGuidePayload struct {
	AssetType    string                   `json:"asset_type"`
	StrategyType string                   `json:"strategy_type"`
	StrategyName string                   `json:"strategy_name"`
	Symbol       string                   `json:"symbol"`
	PriceRef     float64                  `json:"price_ref"`
	BuyRules     []quantitativeRule       `json:"buy_rules"`
	SellRules    []quantitativeRule       `json:"sell_rules"`
	RiskControls quantitativeRiskControls `json:"risk_controls"`
}

var numericPattern = regexp.MustCompile(`[-+]?\d*\.?\d+`)

func (s *Service) attachTradeGuides(output *AgentOutput, screeningOutput *AgentScreeningOutput, screeningResult *ScreeningResult) {
	if output == nil {
		return
	}

	for i := range output.Stocks {
		text, raw := s.buildTradeGuideForStock(&output.Stocks[i], screeningOutput, screeningResult)
		output.Stocks[i].TradeGuideText = text
		output.Stocks[i].TradeGuideJSON = raw
		output.Stocks[i].TradeGuideJSONV2 = convertLegacyGuideToV2(raw)
		output.Stocks[i].TradeGuideVersion = "v1"
	}
}

func buildPersistableGuide(stock *StockPick, pickID string) *OperationGuide {
	if stock == nil || strings.TrimSpace(stock.Symbol) == "" {
		return nil
	}

	guide := &OperationGuide{}
	if stock.OperationGuide != nil {
		copied := *stock.OperationGuide
		guide = &copied
	}
	guide.Symbol = stock.Symbol
	guide.PickID = pickID
	guide.TradeGuideText = strings.TrimSpace(stock.TradeGuideText)
	guide.TradeGuideJSON = strings.TrimSpace(stock.TradeGuideJSON)
	guide.TradeGuideJSONV2 = strings.TrimSpace(stock.TradeGuideJSONV2)
	guide.TradeGuideVersion = strings.TrimSpace(stock.TradeGuideVersion)

	if len(guide.BuyConditions) == 0 && len(guide.SellConditions) == 0 {
		buys, sells, stopLoss, takeProfit := deriveConditionsFromTradeGuide(guide.TradeGuideJSON, guide.TradeGuideText)
		guide.BuyConditions = append(guide.BuyConditions, buys...)
		guide.SellConditions = append(guide.SellConditions, sells...)
		if strings.TrimSpace(guide.StopLoss) == "" {
			guide.StopLoss = stopLoss
		}
		if strings.TrimSpace(guide.TakeProfit) == "" {
			guide.TakeProfit = takeProfit
		}
	}

	if len(guide.BuyConditions) == 0 && strings.TrimSpace(guide.TradeGuideText) != "" {
		guide.BuyConditions = []Condition{{Type: "text", Description: "参考交易指南执行买入逻辑", Value: guide.TradeGuideText}}
	}
	if len(guide.SellConditions) == 0 && strings.TrimSpace(guide.TradeGuideText) != "" {
		guide.SellConditions = []Condition{{Type: "text", Description: "参考交易指南执行卖出逻辑", Value: guide.TradeGuideText}}
	}
	if strings.TrimSpace(guide.TradeGuideVersion) == "" && (guide.TradeGuideText != "" || guide.TradeGuideJSON != "" || guide.TradeGuideJSONV2 != "") {
		guide.TradeGuideVersion = "v1"
	}

	if len(guide.BuyConditions) == 0 && len(guide.SellConditions) == 0 &&
		strings.TrimSpace(guide.StopLoss) == "" && strings.TrimSpace(guide.TakeProfit) == "" &&
		guide.TradeGuideText == "" && guide.TradeGuideJSON == "" && guide.TradeGuideJSONV2 == "" {
		return nil
	}
	return guide
}

func deriveConditionsFromTradeGuide(rawGuide string, fallbackText string) ([]Condition, []Condition, string, string) {
	var parsed stockTradeGuidePayload
	rawGuide = strings.TrimSpace(rawGuide)
	if rawGuide == "" || !json.Valid([]byte(rawGuide)) {
		return nil, nil, "", ""
	}
	if err := json.Unmarshal([]byte(rawGuide), &parsed); err != nil {
		return nil, nil, "", ""
	}

	buys := make([]Condition, 0, len(parsed.BuyRules))
	for _, rule := range parsed.BuyRules {
		buys = append(buys, Condition{
			Type:        "quant",
			Description: "买入规则：" + strings.TrimSpace(rule.Note),
			Value:       buildRuleValue(rule),
		})
	}
	sells := make([]Condition, 0, len(parsed.SellRules))
	for _, rule := range parsed.SellRules {
		sells = append(sells, Condition{
			Type:        "quant",
			Description: "卖出规则：" + strings.TrimSpace(rule.Note),
			Value:       buildRuleValue(rule),
		})
	}

	stopLoss := ""
	takeProfit := ""
	if parsed.RiskControls.StopLossPrice > 0 {
		stopLoss = fmt.Sprintf("跌至%.2f执行止损", parsed.RiskControls.StopLossPrice)
	}
	if parsed.RiskControls.TakeProfitPrice > 0 {
		takeProfit = fmt.Sprintf("涨至%.2f可止盈", parsed.RiskControls.TakeProfitPrice)
	}
	if len(buys) == 0 && len(sells) == 0 && strings.TrimSpace(fallbackText) != "" {
		buys = append(buys, Condition{Type: "text", Description: "参考交易指南", Value: fallbackText})
		sells = append(sells, Condition{Type: "text", Description: "参考交易指南", Value: fallbackText})
	}
	return buys, sells, stopLoss, takeProfit
}

func buildRuleValue(rule quantitativeRule) string {
	parts := []string{
		"indicator=" + strings.TrimSpace(rule.Indicator),
		"operator=" + strings.TrimSpace(rule.Operator),
		fmt.Sprintf("trigger=%.4f", rule.TriggerValue),
	}
	if tf := strings.TrimSpace(rule.Timeframe); tf != "" {
		parts = append(parts, "timeframe="+tf)
	}
	return strings.Join(parts, ";")
}

func (s *Service) buildTradeGuideForStock(stock *StockPick, screeningOutput *AgentScreeningOutput, screeningResult *ScreeningResult) (string, string) {
	if stock == nil {
		return "", "{}"
	}

	strategyType, strategyName, conditions := resolveStrategyMeta(screeningOutput, screeningResult)
	screened := findScreenedStock(stock.Symbol, screeningResult)
	priceRef := resolvePriceReference(stock, screened)
	support, resistance := resolveSupportResistance(stock, priceRef)
	stopLoss, takeProfit := resolveStopLossTakeProfit(stock, priceRef)

	// 防守位不应高于突破位
	if support >= resistance {
		support = roundPrice(priceRef * 0.97)
		resistance = roundPrice(priceRef * 1.02)
	}
	// 止损和止盈兜底
	if stopLoss >= priceRef {
		stopLoss = roundPrice(priceRef * 0.95)
	}
	if takeProfit <= priceRef {
		takeProfit = roundPrice(priceRef * 1.10)
	}

	buyRules := []quantitativeRule{
		{
			RuleID:       "BUY_PRICE_BREAKOUT",
			Indicator:    "price",
			Operator:     ">=",
			TriggerValue: resistance,
			Timeframe:    "daily_close",
			Weight:       0.35,
			Note:         "收盘突破关键压力位后考虑建仓",
		},
	}
	sellRules := []quantitativeRule{
		{
			RuleID:       "SELL_BREAK_SUPPORT",
			Indicator:    "price",
			Operator:     "<=",
			TriggerValue: support,
			Timeframe:    "daily_close",
			Weight:       0.40,
			Note:         "跌破关键支撑位执行减仓或离场",
		},
	}

	if conditions.MA5AboveMA20 != nil && *conditions.MA5AboveMA20 {
		buyRules = append(buyRules, quantitativeRule{
			RuleID:       "BUY_MA_CROSS",
			Indicator:    "ma5_minus_ma20",
			Operator:     ">",
			TriggerValue: 0,
			Timeframe:    "daily_close",
			Weight:       0.22,
			Note:         "MA5上穿并维持在MA20上方",
		})
		sellRules = append(sellRules, quantitativeRule{
			RuleID:       "SELL_MA_CROSS_DOWN",
			Indicator:    "ma5_minus_ma20",
			Operator:     "<=",
			TriggerValue: 0,
			Timeframe:    "daily_close",
			Weight:       0.28,
			Note:         "MA5下穿MA20且趋势转弱",
		})
	}
	if conditions.MA20Rising != nil && *conditions.MA20Rising {
		buyRules = append(buyRules, quantitativeRule{
			RuleID:       "BUY_MA20_SLOPE_UP",
			Indicator:    "ma20_slope",
			Operator:     ">",
			TriggerValue: 0,
			Timeframe:    "daily_close",
			Weight:       0.16,
			Note:         "MA20保持上行，趋势未破坏",
		})
		sellRules = append(sellRules, quantitativeRule{
			RuleID:       "SELL_MA20_SLOPE_DOWN",
			Indicator:    "ma20_slope",
			Operator:     "<=",
			TriggerValue: 0,
			Timeframe:    "daily_close",
			Weight:       0.20,
			Note:         "MA20拐头向下，趋势级别走弱",
		})
	}
	if conditions.MACDGolden != nil && *conditions.MACDGolden {
		buyRules = append(buyRules, quantitativeRule{
			RuleID:       "BUY_MACD_GOLDEN",
			Indicator:    "macd_diff",
			Operator:     ">",
			TriggerValue: 0,
			Timeframe:    "daily_close",
			Weight:       0.18,
			Note:         "MACD快慢线金叉并维持红柱",
		})
		sellRules = append(sellRules, quantitativeRule{
			RuleID:       "SELL_MACD_DEAD",
			Indicator:    "macd_diff",
			Operator:     "<=",
			TriggerValue: 0,
			Timeframe:    "daily_close",
			Weight:       0.22,
			Note:         "MACD死叉或绿柱持续扩大",
		})
	}
	if conditions.RSIOversold != nil && *conditions.RSIOversold {
		buyRules = append(buyRules, quantitativeRule{
			RuleID:       "BUY_RSI_RECOVER",
			Indicator:    "rsi",
			Operator:     ">=",
			TriggerValue: 30,
			Timeframe:    "daily_close",
			Weight:       0.14,
			Note:         "RSI由超卖区回升至30上方",
		})
		sellRules = append(sellRules, quantitativeRule{
			RuleID:       "SELL_RSI_OVERHEAT",
			Indicator:    "rsi",
			Operator:     ">=",
			TriggerValue: 70,
			Timeframe:    "daily_close",
			Weight:       0.18,
			Note:         "RSI进入高位后回落风险增加",
		})
	}
	if conditions.RSIOverbought != nil && *conditions.RSIOverbought {
		buyRules = append(buyRules, quantitativeRule{
			RuleID:       "BUY_RSI_STRONG",
			Indicator:    "rsi",
			Operator:     ">=",
			TriggerValue: 55,
			Timeframe:    "daily_close",
			Weight:       0.10,
			Note:         "RSI维持强势区间，趋势仍有延续性",
		})
		sellRules = append(sellRules, quantitativeRule{
			RuleID:       "SELL_RSI_WEAKEN",
			Indicator:    "rsi",
			Operator:     "<=",
			TriggerValue: 50,
			Timeframe:    "daily_close",
			Weight:       0.16,
			Note:         "RSI跌破中轴，趋势动能明显衰减",
		})
	}
	if conditions.VolumeSpike != nil && *conditions.VolumeSpike {
		buyRules = append(buyRules, quantitativeRule{
			RuleID:       "BUY_VOLUME_CONFIRM",
			Indicator:    "volume_ratio",
			Operator:     ">=",
			TriggerValue: 2.0,
			Timeframe:    "daily_close",
			Weight:       0.15,
			Note:         "放量确认突破有效性",
		})
		sellRules = append(sellRules, quantitativeRule{
			RuleID:       "SELL_VOLUME_FAILURE",
			Indicator:    "volume_ratio",
			Operator:     "<=",
			TriggerValue: 1.0,
			Timeframe:    "daily_close",
			Weight:       0.14,
			Note:         "缩量回落且无法再创新高",
		})
	}

	if len(buyRules) == 1 {
		buyRules = append(buyRules, quantitativeRule{
			RuleID:       "BUY_TREND_CONFIRM",
			Indicator:    "trend_strength",
			Operator:     ">=",
			TriggerValue: 0.6,
			Timeframe:    "daily_close",
			Weight:       0.15,
			Note:         "趋势强度维持中高位",
		})
	}
	if len(sellRules) == 1 {
		sellRules = append(sellRules, quantitativeRule{
			RuleID:       "SELL_TREND_BREAK",
			Indicator:    "trend_strength",
			Operator:     "<=",
			TriggerValue: 0.35,
			Timeframe:    "daily_close",
			Weight:       0.15,
			Note:         "趋势强度快速回落",
		})
	}

	requiredSignals := 2
	if len(buyRules) < 2 {
		requiredSignals = 1
	}
	guideText := fmt.Sprintf(
		"%s策略下建议当买入规则至少%d条同时触发时分批建仓；若任一核心卖出规则触发或跌破止损价则减仓离场。当前量化阈值：突破买入¥%.2f，防守位¥%.2f，止损¥%.2f，止盈¥%.2f。",
		strategyName,
		requiredSignals,
		resistance,
		support,
		stopLoss,
		takeProfit,
	)

	payload := stockTradeGuidePayload{
		AssetType:    "stock",
		StrategyType: strategyType,
		StrategyName: strategyName,
		Symbol:       stock.Symbol,
		PriceRef:     priceRef,
		BuyRules:     buyRules,
		SellRules:    sellRules,
		RiskControls: quantitativeRiskControls{
			StopLossPrice:    stopLoss,
			TakeProfitPrice:  takeProfit,
			MaxPositionRatio: 0.2,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return guideText, "{}"
	}
	return guideText, string(raw)
}

func resolveStrategyMeta(screeningOutput *AgentScreeningOutput, screeningResult *ScreeningResult) (string, string, ScreeningRequest) {
	conditions := ScreeningRequest{}
	strategyType := ""
	strategyName := ""
	if screeningOutput != nil {
		conditions = screeningOutput.ScreeningConditions
		strategyType = strings.TrimSpace(screeningOutput.ScreeningConditions.StrategyType)
		strategyName = strings.TrimSpace(screeningOutput.StrategyName)
	}
	if strategyType == "" && screeningResult != nil {
		strategyType = strings.TrimSpace(screeningResult.StrategyType)
	}
	if strategyType == "" {
		strategyType = "balanced_stock_selection"
	}
	if strategyName == "" {
		strategyName = strategyType
	}
	return strategyType, strategyName, conditions
}

func findScreenedStock(symbol string, screeningResult *ScreeningResult) *ScreenedStock {
	if screeningResult == nil || symbol == "" {
		return nil
	}
	for i := range screeningResult.Stocks {
		if screeningResult.Stocks[i].Symbol == symbol {
			return &screeningResult.Stocks[i]
		}
	}
	return nil
}

func resolvePriceReference(stock *StockPick, screened *ScreenedStock) float64 {
	if stock != nil && stock.CurrentPrice > 0 {
		return roundPrice(stock.CurrentPrice)
	}
	if screened != nil && screened.Price > 0 {
		return roundPrice(screened.Price)
	}
	return 1
}

func resolveSupportResistance(stock *StockPick, priceRef float64) (float64, float64) {
	supportCandidates := []float64{}
	resistanceCandidates := []float64{}

	if stock != nil {
		for _, level := range stock.TechnicalReasons.KeyLevels {
			value, ok := parseFirstNumber(level)
			if !ok || value <= 0 {
				continue
			}
			lower := strings.ToLower(level)
			if strings.Contains(lower, "支撑") {
				supportCandidates = append(supportCandidates, value)
			}
			if strings.Contains(lower, "压力") {
				resistanceCandidates = append(resistanceCandidates, value)
			}
		}

		if stock.OperationGuide != nil {
			for _, cond := range stock.OperationGuide.BuyConditions {
				value, ok := parseConditionValue(cond)
				if !ok {
					continue
				}
				lower := strings.ToLower(cond.Description)
				if strings.Contains(lower, "突破") || strings.Contains(lower, "压力") {
					resistanceCandidates = append(resistanceCandidates, value)
				}
				if strings.Contains(lower, "支撑") || strings.Contains(lower, "回调") {
					supportCandidates = append(supportCandidates, value)
				}
			}
			for _, cond := range stock.OperationGuide.SellConditions {
				value, ok := parseConditionValue(cond)
				if !ok {
					continue
				}
				lower := strings.ToLower(cond.Description)
				if strings.Contains(lower, "跌破") || strings.Contains(lower, "支撑") {
					supportCandidates = append(supportCandidates, value)
				}
				if strings.Contains(lower, "突破") || strings.Contains(lower, "压力") {
					resistanceCandidates = append(resistanceCandidates, value)
				}
			}
		}
	}

	support, okSupport := chooseSupportLevel(supportCandidates, priceRef)
	resistance, okResistance := chooseResistanceLevel(resistanceCandidates, priceRef)
	if !okSupport {
		support = roundPrice(priceRef * 0.97)
	}
	if !okResistance {
		resistance = roundPrice(priceRef * 1.02)
	}
	return support, resistance
}

func resolveStopLossTakeProfit(stock *StockPick, priceRef float64) (float64, float64) {
	stopLoss := roundPrice(priceRef * 0.95)
	takeProfit := roundPrice(priceRef * 1.10)
	if stock == nil || stock.OperationGuide == nil {
		return stopLoss, takeProfit
	}

	if value, ok := parseFirstNumber(stock.OperationGuide.StopLoss); ok && value > 0 {
		stopLoss = roundPrice(value)
	}
	if value, ok := parseFirstNumber(stock.OperationGuide.TakeProfit); ok && value > 0 {
		takeProfit = roundPrice(value)
	}
	return stopLoss, takeProfit
}

func chooseSupportLevel(candidates []float64, priceRef float64) (float64, bool) {
	best := 0.0
	found := false
	for _, c := range candidates {
		if c <= 0 {
			continue
		}
		if c <= priceRef {
			if !found || c > best {
				best = c
				found = true
			}
		}
	}
	if found {
		return roundPrice(best), true
	}

	for _, c := range candidates {
		if c > 0 {
			return roundPrice(c), true
		}
	}
	return 0, false
}

func chooseResistanceLevel(candidates []float64, priceRef float64) (float64, bool) {
	best := 0.0
	found := false
	for _, c := range candidates {
		if c <= 0 {
			continue
		}
		if c >= priceRef {
			if !found || c < best {
				best = c
				found = true
			}
		}
	}
	if found {
		return roundPrice(best), true
	}

	for _, c := range candidates {
		if c > 0 {
			return roundPrice(c), true
		}
	}
	return 0, false
}

func parseConditionValue(cond Condition) (float64, bool) {
	if v, ok := parseFirstNumber(cond.Value); ok {
		return v, true
	}
	return parseFirstNumber(cond.Description)
}

func parseFirstNumber(raw string) (float64, bool) {
	cleaned := strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if cleaned == "" {
		return 0, false
	}
	token := numericPattern.FindString(cleaned)
	if token == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func roundPrice(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Round(v*100) / 100
}

// buildWarnings 构建警告信息
func (s *Service) buildWarnings(
	marketData MarketData,
	newsEvents []NewsEvent,
	stockList []tool.MarketItem,
) []string {
	warnings := []string{}

	// 市场风险提示
	if marketData.DownCount > marketData.UpCount*2 {
		warnings = append(warnings, "市场下跌家数较多，注意系统性风险")
	}

	// 新闻风险提示
	highPriorityCount := 0
	for _, n := range newsEvents {
		if n.Priority >= 4 {
			highPriorityCount++
		}
	}
	if highPriorityCount > 10 {
		warnings = append(warnings, "近期重大事件较多，市场波动可能加大")
	}

	// 流动性风险
	lowLiquidityCount := 0
	for _, stock := range stockList {
		if stock.Amount < 1e7 {
			lowLiquidityCount++
		}
	}
	if len(stockList) > 0 && float64(lowLiquidityCount)/float64(len(stockList)) > 0.3 {
		warnings = append(warnings, "市场流动性整体偏低，注意交易风险")
	}

	return warnings
}
