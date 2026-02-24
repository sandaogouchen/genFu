package stockpicker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"genFu/internal/tool"
)

// 技术筛选的最大候选股票数量，避免请求过多K线数据
const maxTechnicalFilterCount = 100

// StockScreenerTool 股票筛选工具
type StockScreenerTool struct {
	registry *tool.Registry
}

// NewStockScreenerTool 创建股票筛选工具
func NewStockScreenerTool(registry *tool.Registry) *StockScreenerTool {
	return &StockScreenerTool{registry: registry}
}

// Spec 返回工具规格
func (t *StockScreenerTool) Spec() tool.ToolSpec {
	return tool.ToolSpec{
		Name:        "stock_screener",
		Description: "screen stocks from full market based on quantitative conditions",
		Params: map[string]string{
			"action":          "string",
			"strategy_type":   "string",
			"price_min":       "number",
			"price_max":       "number",
			"change_rate_min": "number",
			"change_rate_max": "number",
			"amount_min":      "number",
			"amount_max":      "number",
			"ma5_above_ma20":  "boolean",
			"ma20_rising":     "boolean",
			"macd_golden":     "boolean",
			"rsi_oversold":    "boolean",
			"rsi_overbought":  "boolean",
			"volume_spike":    "boolean",
			"limit":           "number",
		},
		Required: []string{"action"},
	}
}

// Execute 执行工具
func (t *StockScreenerTool) Execute(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	if t.registry == nil {
		return tool.ToolResult{Name: "stock_screener", Error: "registry_not_initialized"}, errors.New("registry_not_initialized")
	}

	action, err := t.requireString(args, "action")
	if err != nil {
		return tool.ToolResult{Name: "stock_screener", Error: err.Error()}, err
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "screen":
		return t.executeScreen(ctx, args)
	case "list_strategies":
		return t.listStrategies()
	default:
		return tool.ToolResult{Name: "stock_screener", Error: "unsupported_action"}, errors.New("unsupported_action")
	}
}

// executeScreen 执行筛选
func (t *StockScreenerTool) executeScreen(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	startTime := time.Now()

	// 1. 解析筛选条件
	req := t.parseScreeningRequest(args)
	log.Printf("[筛选工具] 开始筛选 strategy=%s limit=%d", req.StrategyType, req.Limit)

	// 2. 获取全市场股票列表
	stockList, err := t.getStockList(ctx)
	if err != nil {
		return tool.ToolResult{Name: "stock_screener", Error: err.Error()}, err
	}
	log.Printf("[筛选工具] 获取股票列表 count=%d 耗时=%v", len(stockList), time.Since(startTime))

	// 3. 第一阶段：基础条件筛选
	basicStartTime := time.Now()
	basicFiltered := t.applyBasicFilters(stockList, req)
	appliedFilters := t.buildAppliedFilters(req)
	log.Printf("[筛选工具] 基础筛选完成 输入=%d 输出=%d 耗时=%v", len(stockList), len(basicFiltered), time.Since(basicStartTime))

	// 4. 第二阶段：技术指标筛选（如果有技术条件）
	var result []ScreenedStock
	if t.hasTechnicalConditions(req) {
		// 限制技术筛选的股票数量，避免请求过多K线数据
		techInput := basicFiltered
		if len(techInput) > maxTechnicalFilterCount {
			techInput = techInput[:maxTechnicalFilterCount]
			log.Printf("[筛选工具] 限制技术筛选数量为%d", maxTechnicalFilterCount)
		}

		techStartTime := time.Now()
		techFiltered, err := t.applyTechnicalFilters(ctx, techInput, req)
		if err != nil {
			// 技术指标筛选失败，使用基础筛选结果
			log.Printf("[筛选工具] 技术筛选失败 err=%v", err)
			result = t.convertToScreenedStocks(basicFiltered)
		} else {
			result = techFiltered
			log.Printf("[筛选工具] 技术筛选完成 输入=%d 输出=%d 耗时=%v", len(techInput), len(result), time.Since(techStartTime))
		}
	} else {
		result = t.convertToScreenedStocks(basicFiltered)
	}

	// 5. 限制返回数量
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if len(result) > req.Limit {
		result = result[:req.Limit]
	}

	// 6. 构建筛选结果
	screeningResult := ScreeningResult{
		StrategyType:   req.StrategyType,
		TotalMatched:   len(result),
		ReturnedCount:  len(result),
		Stocks:         result,
		AppliedFilters: appliedFilters,
		ScreenedAt:     time.Now(),
	}

	log.Printf("[筛选工具] 筛选完成 总耗时=%v 结果数量=%d", time.Since(startTime), len(result))

	return tool.ToolResult{
		Name:   "stock_screener",
		Output: screeningResult,
	}, nil
}

// parseScreeningRequest 解析筛选请求
func (t *StockScreenerTool) parseScreeningRequest(args map[string]interface{}) ScreeningRequest {
	req := ScreeningRequest{
		Limit: 50,
	}

	if v, ok := args["strategy_type"].(string); ok {
		req.StrategyType = v
	}
	if v, ok := args["limit"].(float64); ok && v > 0 {
		req.Limit = int(v)
	}
	if v, ok := args["limit"].(int); ok && v > 0 {
		req.Limit = v
	}

	// 价格条件
	if v, ok := args["price_min"].(float64); ok {
		req.PriceMin = &v
	}
	if v, ok := args["price_max"].(float64); ok {
		req.PriceMax = &v
	}

	// 涨跌幅条件
	if v, ok := args["change_rate_min"].(float64); ok {
		req.ChangeRateMin = &v
	}
	if v, ok := args["change_rate_max"].(float64); ok {
		req.ChangeRateMax = &v
	}

	// 成交额条件
	if v, ok := args["amount_min"].(float64); ok {
		req.AmountMin = &v
	}
	if v, ok := args["amount_max"].(float64); ok {
		req.AmountMax = &v
	}

	// 技术指标条件
	if v, ok := args["ma5_above_ma20"].(bool); ok {
		req.MA5AboveMA20 = &v
	}
	if v, ok := args["ma20_rising"].(bool); ok {
		req.MA20Rising = &v
	}
	if v, ok := args["macd_golden"].(bool); ok {
		req.MACDGolden = &v
	}
	if v, ok := args["rsi_oversold"].(bool); ok {
		req.RSIOversold = &v
	}
	if v, ok := args["rsi_overbought"].(bool); ok {
		req.RSIOverbought = &v
	}
	if v, ok := args["volume_spike"].(bool); ok {
		req.VolumeSpike = &v
	}

	return req
}

// getStockList 获取股票列表
func (t *StockScreenerTool) getStockList(ctx context.Context) ([]tool.MarketItem, error) {
	result, err := t.registry.Execute(ctx, tool.ToolCall{
		Name: "eastmoney",
		Arguments: map[string]interface{}{
			"action":    "get_stock_list",
			"page":      1,
			"page_size": 5000,
		},
	})
	if err != nil {
		return nil, err
	}

	if items, ok := result.Output.([]tool.MarketItem); ok {
		return items, nil
	}
	return nil, errors.New("unexpected_stock_list_type")
}

// applyBasicFilters 应用基础条件筛选
func (t *StockScreenerTool) applyBasicFilters(stocks []tool.MarketItem, req ScreeningRequest) []tool.MarketItem {
	filtered := make([]tool.MarketItem, 0)

	for _, stock := range stocks {
		// 跳过无效数据
		if stock.Price <= 0 {
			continue
		}

		// 价格筛选
		if req.PriceMin != nil && stock.Price < *req.PriceMin {
			continue
		}
		if req.PriceMax != nil && stock.Price > *req.PriceMax {
			continue
		}

		// 涨跌幅筛选
		if req.ChangeRateMin != nil && stock.ChangeRate < *req.ChangeRateMin {
			continue
		}
		if req.ChangeRateMax != nil && stock.ChangeRate > *req.ChangeRateMax {
			continue
		}

		// 成交额筛选
		if req.AmountMin != nil && stock.Amount < *req.AmountMin {
			continue
		}
		if req.AmountMax != nil && stock.Amount > *req.AmountMax {
			continue
		}

		filtered = append(filtered, stock)
	}

	return filtered
}

// hasTechnicalConditions 检查是否有技术指标筛选条件
func (t *StockScreenerTool) hasTechnicalConditions(req ScreeningRequest) bool {
	return req.MA5AboveMA20 != nil ||
		req.MA20Rising != nil ||
		req.MACDGolden != nil ||
		req.RSIOversold != nil ||
		req.RSIOverbought != nil ||
		req.VolumeSpike != nil
}

// applyTechnicalFilters 应用技术指标筛选
func (t *StockScreenerTool) applyTechnicalFilters(ctx context.Context, stocks []tool.MarketItem, req ScreeningRequest) ([]ScreenedStock, error) {
	result := make([]ScreenedStock, 0)

	// 并发获取K线数据，但限制并发数
	const maxConcurrent = 10
	sem := make(chan struct{}, maxConcurrent)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for _, stock := range stocks {
		wg.Add(1)
		go func(s tool.MarketItem) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			// 获取K线数据
			kline, err := t.getKline(ctx, s.Code)
			if err != nil {
				return
			}

			// 计算技术指标
			prices, volumes := extractPricesVolumes(kline)
			if len(prices) < 30 {
				return
			}

			techInfo := CalculateAllIndicators(prices, volumes)
			if techInfo == nil {
				return
			}

			// 检查技术条件
			if !t.matchTechnicalConditions(techInfo, req) {
				return
			}

			// 构建匹配原因
			reasons := t.buildMatchReasons(s, techInfo, req)

			screened := ScreenedStock{
				Symbol:        s.Code,
				Name:          s.Name,
				Price:         s.Price,
				ChangeRate:    s.ChangeRate,
				Amount:        s.Amount,
				Amplitude:     s.Amplitude,
				MatchScore:    1.0,
				MatchReasons:  reasons,
				TechnicalInfo: techInfo,
			}

			mu.Lock()
			result = append(result, screened)
			if firstErr == nil && err != nil {
				firstErr = err
			}
			mu.Unlock()
		}(stock)
	}

	wg.Wait()

	return result, firstErr
}

// getKline 获取K线数据
func (t *StockScreenerTool) getKline(ctx context.Context, code string) ([]tool.KlinePoint, error) {
	result, err := t.registry.Execute(ctx, tool.ToolCall{
		Name: "marketdata",
		Arguments: map[string]interface{}{
			"action": "get_stock_kline",
			"code":   code,
			"days":   60, // 获取60天数据用于计算技术指标
		},
	})
	if err != nil {
		return nil, err
	}

	if points, ok := result.Output.([]tool.KlinePoint); ok {
		return points, nil
	}
	return nil, errors.New("unexpected_kline_type")
}

// matchTechnicalConditions 检查技术条件
func (t *StockScreenerTool) matchTechnicalConditions(tech *TechnicalInfo, req ScreeningRequest) bool {
	// MA5上穿MA20
	if req.MA5AboveMA20 != nil && *req.MA5AboveMA20 {
		if !IsMA5AboveMA20(tech.MA5, tech.MA20) {
			return false
		}
	}

	// MA20向上 (简化处理，使用MA10 > MA20作为代理)
	if req.MA20Rising != nil && *req.MA20Rising {
		if tech.MA10 < tech.MA20 {
			return false
		}
	}

	// MACD金叉
	if req.MACDGolden != nil && *req.MACDGolden {
		if tech.MACD <= tech.MACDSignal {
			return false
		}
	}

	// RSI超卖
	if req.RSIOversold != nil && *req.RSIOversold {
		if tech.RSI >= 30 {
			return false
		}
	}

	// RSI超买
	if req.RSIOverbought != nil && *req.RSIOverbought {
		if tech.RSI <= 70 {
			return false
		}
	}

	// 放量
	if req.VolumeSpike != nil && *req.VolumeSpike {
		if tech.VolumeRatio < 2.0 {
			return false
		}
	}

	return true
}

// extractPricesVolumes 从K线数据提取价格和成交量
func extractPricesVolumes(kline []tool.KlinePoint) ([]float64, []float64) {
	prices := make([]float64, 0, len(kline))
	volumes := make([]float64, 0, len(kline))

	for _, p := range kline {
		prices = append(prices, p.Close)
		volumes = append(volumes, p.Volume)
	}

	return prices, volumes
}

// convertToScreenedStocks 转换为筛选后的股票
func (t *StockScreenerTool) convertToScreenedStocks(stocks []tool.MarketItem) []ScreenedStock {
	result := make([]ScreenedStock, 0, len(stocks))
	for _, s := range stocks {
		reasons := []string{}
		if s.ChangeRate > 0 {
			reasons = append(reasons, "上涨趋势")
		}
		if s.Amount > 1e8 {
			reasons = append(reasons, "成交活跃")
		}

		result = append(result, ScreenedStock{
			Symbol:       s.Code,
			Name:         s.Name,
			Price:        s.Price,
			ChangeRate:   s.ChangeRate,
			Amount:       s.Amount,
			Amplitude:    s.Amplitude,
			MatchScore:   1.0,
			MatchReasons: reasons,
		})
	}
	return result
}

// buildMatchReasons 构建匹配原因
func (t *StockScreenerTool) buildMatchReasons(s tool.MarketItem, tech *TechnicalInfo, req ScreeningRequest) []string {
	reasons := []string{}

	if s.ChangeRate > 0 {
		reasons = append(reasons, fmt.Sprintf("涨幅%.2f%%", s.ChangeRate))
	}
	if s.Amount > 1e8 {
		reasons = append(reasons, "成交活跃")
	}
	if req.MA5AboveMA20 != nil && *req.MA5AboveMA20 && IsMA5AboveMA20(tech.MA5, tech.MA20) {
		reasons = append(reasons, "MA5上穿MA20")
	}
	if req.MACDGolden != nil && *req.MACDGolden && tech.MACD > tech.MACDSignal {
		reasons = append(reasons, "MACD金叉")
	}
	if req.RSIOversold != nil && *req.RSIOversold {
		reasons = append(reasons, "RSI超卖")
	}
	if req.VolumeSpike != nil && *req.VolumeSpike && tech.VolumeRatio > 2.0 {
		reasons = append(reasons, fmt.Sprintf("量比%.2f", tech.VolumeRatio))
	}

	return reasons
}

// buildAppliedFilters 构建已应用的筛选条件描述
func (t *StockScreenerTool) buildAppliedFilters(req ScreeningRequest) []string {
	filters := []string{}

	if req.PriceMin != nil {
		filters = append(filters, fmt.Sprintf("价格>=%.2f", *req.PriceMin))
	}
	if req.PriceMax != nil {
		filters = append(filters, fmt.Sprintf("价格<=%.2f", *req.PriceMax))
	}
	if req.ChangeRateMin != nil {
		filters = append(filters, fmt.Sprintf("涨幅>=%.2f%%", *req.ChangeRateMin))
	}
	if req.ChangeRateMax != nil {
		filters = append(filters, fmt.Sprintf("涨幅<=%.2f%%", *req.ChangeRateMax))
	}
	if req.AmountMin != nil {
		filters = append(filters, fmt.Sprintf("成交额>=%.0f", *req.AmountMin))
	}
	if req.AmountMax != nil {
		filters = append(filters, fmt.Sprintf("成交额<=%.0f", *req.AmountMax))
	}
	if req.MA5AboveMA20 != nil && *req.MA5AboveMA20 {
		filters = append(filters, "MA5上穿MA20")
	}
	if req.MA20Rising != nil && *req.MA20Rising {
		filters = append(filters, "MA20向上")
	}
	if req.MACDGolden != nil && *req.MACDGolden {
		filters = append(filters, "MACD金叉")
	}
	if req.RSIOversold != nil && *req.RSIOversold {
		filters = append(filters, "RSI超卖")
	}
	if req.RSIOverbought != nil && *req.RSIOverbought {
		filters = append(filters, "RSI超买")
	}
	if req.VolumeSpike != nil && *req.VolumeSpike {
		filters = append(filters, "放量")
	}

	return filters
}

// listStrategies 列出可用策略
func (t *StockScreenerTool) listStrategies() (tool.ToolResult, error) {
	strategies := make([]map[string]interface{}, 0, len(strategyOrder))
	defaultCtx := strategyContext{UpRatio: 0.5}
	for _, name := range strategyOrder {
		meta, ok := strategyMetaMap[name]
		if !ok || meta.Builder == nil {
			continue
		}
		conditions := meta.Builder(defaultCtx)
		conditions["strategy_type"] = meta.Name
		strategies = append(strategies, map[string]interface{}{
			"name":        meta.Name,
			"description": meta.Description,
			"conditions":  conditions,
		})
	}

	return tool.ToolResult{
		Name:   "stock_screener",
		Output: map[string]interface{}{"strategies": strategies},
	}, nil
}

// 辅助函数
func (t *StockScreenerTool) requireString(args map[string]interface{}, key string) (string, error) {
	if args == nil {
		return "", errors.New("missing_args")
	}
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing_%s", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("invalid_%s", key)
	}
	return s, nil
}
