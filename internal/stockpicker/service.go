package stockpicker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
		if output.Stocks[i].OperationGuide != nil {
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

	var output AgentOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return nil, fmt.Errorf("json_parse_error: %w", err)
	}

	return &output, nil
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
