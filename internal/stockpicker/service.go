package stockpicker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
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
	screenerAgent     agent.Agent // 筛选Agent：生成筛选策略
	analyzerAgent     agent.Agent // 分析Agent：深度分析筛选结果
	registry          *tool.Registry
	provider          DataProvider
	allocationService *AllocationService
	guideRepo         *GuideRepository
}

// NewService 创建选股服务
func NewService(
	screenerAgent agent.Agent,
	analyzerAgent agent.Agent,
	registry *tool.Registry,
	provider DataProvider,
	guideRepo *GuideRepository,
) *Service {
	return &Service{
		screenerAgent:     screenerAgent,
		analyzerAgent:     analyzerAgent,
		registry:          registry,
		provider:          provider,
		allocationService: NewAllocationService(),
		guideRepo:         guideRepo,
	}
}

// PickStocks 执行选股
func (s *Service) PickStocks(ctx context.Context, req StockPickRequest) (StockPickResponse, error) {
	totalStartTime := time.Now()

	if s == nil {
		return StockPickResponse{}, errors.New("service_not_initialized")
	}

	// 1. 设置默认值
	if req.TopN <= 0 {
		req.TopN = 5
	}
	if req.DateTo.IsZero() {
		req.DateTo = time.Now()
	}
	if req.DateFrom.IsZero() {
		req.DateFrom = req.DateTo.AddDate(0, 0, -3) // 默认近3天
	}
	days := int(req.DateTo.Sub(req.DateFrom).Hours()/24) + 1
	if days <= 0 {
		days = 3
	}

	log.Printf("[选股服务] 开始执行选股 topN=%d days=%d", req.TopN, days)

	// 2. 准备数据
	dataStartTime := time.Now()
	marketData, err := s.provider.GetMarketData(ctx, days)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("get_market_data_failed: %w", err)
	}

	newsEvents, err := s.provider.GetRecentNews(ctx, days, 50)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("get_news_failed: %w", err)
	}

	var holdings []Position
	if req.AccountID > 0 {
		holdings, err = s.provider.GetHoldings(ctx, req.AccountID)
		if err != nil {
			// 持仓获取失败不影响选股
			holdings = []Position{}
		}
	}

	stockList, err := s.provider.GetStockList(ctx)
	if err != nil {
		log.Printf("[选股服务] 获取股票列表失败，降级继续 err=%v", err)
		stockList = []tool.MarketItem{}
	}
	log.Printf("[选股服务] 数据准备完成 耗时=%v 新闻数=%d 股票数=%d", time.Since(dataStartTime), len(newsEvents), len(stockList))

	// ========== 阶段1: 筛选阶段 ==========
	log.Printf("[选股服务] ========== 阶段1: 筛选阶段 ==========")

	// 3. 调用筛选Agent，获取筛选策略
	screenerStartTime := time.Now()
	screenerInput := s.buildScreenerInput(req, marketData, newsEvents, holdings)
	log.Printf("[选股服务] 调用筛选Agent...")
	screenerResp, err := s.screenerAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{
			{Role: message.RoleUser, Content: screenerInput},
		},
	})
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("screener_agent_failed: %w", err)
	}
	log.Printf("[选股服务] 筛选Agent完成 耗时=%v", time.Since(screenerStartTime))

	// 4. 解析筛选策略输出
	screeningOutput, err := s.parseScreenerOutput(screenerResp.Message.Content)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("parse_screener_output_failed: %w", err)
	}
	log.Printf("[选股服务] 筛选策略: %s", screeningOutput.StrategyName)

	// 5. 执行筛选，获取候选股票
	screeningStartTime := time.Now()
	screeningResult, err := s.executeScreening(ctx, screeningOutput.ScreeningConditions)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("execute_screening_failed: %w", err)
	}
	log.Printf("[选股服务] 筛选执行完成 候选股票数=%d 耗时=%v", screeningResult.TotalMatched, time.Since(screeningStartTime))

	// ========== 阶段2: 分析验证阶段 ==========
	log.Printf("[选股服务] ========== 阶段2: 分析验证阶段 ==========")

	// 6. 调用分析验证Agent，对筛选结果进行深度分析
	analyzerStartTime := time.Now()
	analyzerInput := s.buildAnalyzerInput(req, marketData, newsEvents, holdings, screeningResult, screeningOutput)
	log.Printf("[选股服务] 调用分析Agent...")
	analyzerResp, err := s.analyzerAgent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{
			{Role: message.RoleUser, Content: analyzerInput},
		},
	})
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("analyzer_agent_failed: %w", err)
	}
	log.Printf("[选股服务] 分析Agent完成 耗时=%v", time.Since(analyzerStartTime))

	// 7. 解析分析结果
	output, err := s.parseAgentOutput(analyzerResp.Message.Content)
	if err != nil {
		return StockPickResponse{}, fmt.Errorf("parse_output_failed: %w", err)
	}
	s.attachTradeGuides(output, screeningOutput, screeningResult)

	// 8. 补充资产配置信息
	for i := range output.Stocks {
		allocation := s.allocationService.CalculateAllocation(
			&output.Stocks[i],
			holdings,
			stockList,
		)
		output.Stocks[i].Allocation = allocation
	}

	// 9. 限制返回数量
	if len(output.Stocks) > req.TopN {
		output.Stocks = output.Stocks[:req.TopN]
	}

	// 10. 存储操作指南到数据库
	pickID := fmt.Sprintf("pick_%d", time.Now().Unix())
	for i := range output.Stocks {
		if output.Stocks[i].OperationGuide != nil && s.guideRepo != nil {
			output.Stocks[i].OperationGuide.Symbol = output.Stocks[i].Symbol
			output.Stocks[i].OperationGuide.PickID = pickID
			// 设置有效期（默认30天）
			validUntil := time.Now().AddDate(0, 0, 30)
			output.Stocks[i].OperationGuide.ValidUntil = &validUntil
			if err := s.guideRepo.SaveGuide(ctx, output.Stocks[i].OperationGuide); err != nil {
				// 存储失败不影响返回结果，仅记录日志
				fmt.Printf("save operation guide failed: %v\n", err)
			}
		}
	}

	// 11. 构建响应
	log.Printf("[选股服务] 选股完成 最终股票数=%d 总耗时=%v", len(output.Stocks), time.Since(totalStartTime))
	return StockPickResponse{
		PickID:        pickID,
		GeneratedAt:   time.Now(),
		Stocks:        output.Stocks,
		MarketData:    marketData,
		NewsSummary:   output.MarketView,
		Warnings:      s.buildWarnings(marketData, newsEvents, stockList),
		ScreeningInfo: screeningResult,
	}, nil
}

// buildScreenerInput 构建筛选Agent输入
func (s *Service) buildScreenerInput(
	req StockPickRequest,
	marketData MarketData,
	newsEvents []NewsEvent,
	holdings []Position,
) string {
	payload := map[string]interface{}{
		"request":     req,
		"market_data": marketData,
		"news_events": newsEvents,
		"holdings":    holdings,
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
) string {
	payload := map[string]interface{}{
		"request":            req,
		"market_data":        marketData,
		"news_events":        newsEvents,
		"holdings":           holdings,
		"screening_result":   screeningResult,
		"screening_strategy": screeningOutput,
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
		output.Stocks[i].TradeGuideVersion = "v1"
	}
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
