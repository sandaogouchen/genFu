package stockpicker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"genFu/internal/analyze"
	"genFu/internal/financial"
	"genFu/internal/investment"
	"genFu/internal/news"
	"genFu/internal/tool"
)

var canonicalMarketIndexes = []string{
	"上证指数",
	"深证成指",
	"创业板指",
	"北证50",
	"科创50",
	"中证500",
	"中证1000",
	"沪深300",
	"恒生指数",
	"纳斯达克指数",
	"道琼斯指数",
	"美元指数",
	"期货连续",
}

var marketIndexAliases = map[string]string{
	"上证指数":   "上证指数",
	"深证指数":   "深证成指",
	"深证成指":   "深证成指",
	"创业板指":   "创业板指",
	"北证50指数": "北证50",
	"北证50":   "北证50",
	"科创50":   "科创50",
	"中证500":  "中证500",
	"中证1000": "中证1000",
	"沪深300":  "沪深300",
	"恒生指数":   "恒生指数",
	"纳斯达克":   "纳斯达克指数",
	"纳斯达克指数": "纳斯达克指数",
	"道琼斯":    "道琼斯指数",
	"道琼斯指数":  "道琼斯指数",
	"美元指数":   "美元指数",
	"期货连续":   "期货连续",
}

var (
	riseFallPattern  = regexp.MustCompile(`上涨\s*([0-9]+)\s*\([^)]*\)\s*停牌\s*([0-9]+)\s*\([^)]*\)\s*下跌\s*([0-9]+)\s*\([^)]*\)`)
	riseCountPattern = regexp.MustCompile(`上涨\s*([0-9,，]+)`)
	downCountPattern = regexp.MustCompile(`下跌\s*([0-9,，]+)`)
)

// DataProvider 数据提供者接口
type DataProvider interface {
	// GetMarketData 获取近N天大盘数据
	GetMarketData(ctx context.Context, days int) (MarketData, error)

	// GetRecentNews 获取近N天重大新闻
	GetRecentNews(ctx context.Context, days int, limit int) ([]NewsEvent, error)

	// GetHoldings 获取用户持仓
	GetHoldings(ctx context.Context, accountID int64) ([]Position, error)

	// GetStockList 获取全市场股票列表
	GetStockList(ctx context.Context) ([]tool.MarketItem, error)

	// GetFinancialData 获取股票财务数据
	GetFinancialData(ctx context.Context, symbol string) (map[string]interface{}, error)
}

// DefaultDataProvider 默认数据提供者实现
type DefaultDataProvider struct {
	newsRepo     *news.Repository
	investRepo   *investment.Repository
	analyzeRepo  *analyze.Repository
	registry     *tool.Registry
	location     *time.Location
	financialSvc *financial.Service
}

// NewDataProvider 创建数据提供者
func NewDataProvider(
	newsRepo *news.Repository,
	investRepo *investment.Repository,
	analyzeRepo *analyze.Repository,
	registry *tool.Registry,
	financialSvc *financial.Service,
) *DefaultDataProvider {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	return &DefaultDataProvider{
		newsRepo:     newsRepo,
		investRepo:   investRepo,
		analyzeRepo:  analyzeRepo,
		registry:     registry,
		location:     loc,
		financialSvc: financialSvc,
	}
}

// GetMarketData 获取大盘数据
// 优先使用每日复盘报告的数据，如果没有则从实时接口获取
func (p *DefaultDataProvider) GetMarketData(ctx context.Context, days int) (MarketData, error) {
	if p == nil || p.registry == nil {
		return MarketData{}, errors.New("provider_not_initialized")
	}

	marketData := MarketData{
		IndexQuotes: []IndexQuote{},
	}

	// 优先尝试从每日复盘报告获取数据
	if p.analyzeRepo != nil {
		report, err := p.analyzeRepo.GetLatestDailyReviewReport(ctx)
		if err == nil && report.ID > 0 {
			// 检查报告是否是今天的（交易日15:30后生成）
			if p.isReportFresh(report.CreatedAt) {
				log.Printf("[选股数据] 使用每日复盘报告数据 report_id=%d created_at=%s", report.ID, report.CreatedAt.Format("2006-01-02 15:04"))
				marketData = p.extractMarketDataFromReport(report)
				if len(marketData.IndexQuotes) >= 3 {
					return marketData, nil
				}
			}
		}
	}

	// 回退到实时接口获取
	log.Printf("[选股数据] 每日复盘报告不可用，使用实时接口获取")
	return p.getMarketDataFromAPI(ctx)
}

// isReportFresh 检查报告是否新鲜（当天15:30后生成的报告视为有效）
func (p *DefaultDataProvider) isReportFresh(createdAt time.Time) bool {
	now := time.Now().In(p.location)
	reportTime := createdAt.In(p.location)

	// 如果现在是15:30之前，接受昨天15:30之后的报告
	// 如果现在是15:30之后，只接受今天15:30之后的报告
	today1530 := time.Date(now.Year(), now.Month(), now.Day(), 15, 30, 0, 0, p.location)

	if now.Before(today1530) {
		// 现在15:30之前，接受昨天15:30之后的报告
		yesterday1530 := today1530.AddDate(0, 0, -1)
		return reportTime.After(yesterday1530)
	}

	// 现在15:30之后，只接受今天15:30之后的报告
	return reportTime.After(today1530) || reportTime.Equal(today1530)
}

// extractMarketDataFromReport 从每日复盘报告中提取大盘数据
func (p *DefaultDataProvider) extractMarketDataFromReport(report analyze.DailyReviewReport) MarketData {
	marketData := MarketData{
		IndexQuotes: []IndexQuote{},
	}

	// DailyReview报告实际结构是 {"date":"...","data":{...}}
	payload := unwrapDailyReviewPayload(report.Request)

	// 优先使用同花顺复盘中的指数（可覆盖港股/美股等）
	if quotes := extractFupanIndexQuotes(payload); len(quotes) > 0 {
		marketData.IndexQuotes = quotes
	}

	// 兜底：使用每日复盘中的 indexes 字段（加白名单过滤，避免混入个股）
	if len(marketData.IndexQuotes) == 0 {
		marketData.IndexQuotes = extractDailyReviewIndexes(payload)
	}

	// 提取市场统计指标（东财计算值）
	if metrics, ok := payload["market_metrics"].(map[string]interface{}); ok {
		if up, ok := toInt(metrics["up"]); ok {
			marketData.UpCount = up
		}
		if down, ok := toInt(metrics["down"]); ok {
			marketData.DownCount = down
		}
		if limitUp, ok := toInt(metrics["limit_up"]); ok {
			marketData.LimitUp = limitUp
		}
		if limitDown, ok := toInt(metrics["limit_down"]); ok {
			marketData.LimitDown = limitDown
		}
		log.Printf("[选股数据] market_metrics来源 up=%d down=%d limit_up=%d limit_down=%d", marketData.UpCount, marketData.DownCount, marketData.LimitUp, marketData.LimitDown)
	}

	// 使用同花顺“个股涨跌图”覆盖涨跌家数
	if up, down, ok := extractFupanRiseDown(payload); ok {
		log.Printf("[选股数据] fupan_report覆盖涨跌家数 up=%d down=%d", up, down)
		marketData.UpCount = up
		marketData.DownCount = down
	} else {
		log.Printf("[选股数据] fupan_report涨跌家数不可用，保留market_metrics up=%d down=%d", marketData.UpCount, marketData.DownCount)
	}

	fillMarketSentiment(&marketData)
	return marketData
}

// getMarketDataFromAPI 从API实时获取大盘数据
func (p *DefaultDataProvider) getMarketDataFromAPI(ctx context.Context) (MarketData, error) {
	marketData := MarketData{
		IndexQuotes: []IndexQuote{},
	}

	// 获取主要指数行情（使用明确前缀，避免000001被识别成平安银行）
	indexes := []struct {
		Code      string
		Name      string
		FetchCode string
	}{
		{Code: "000001", Name: "上证指数", FetchCode: "SH000001"},
		{Code: "399001", Name: "深证成指", FetchCode: "SZ399001"},
		{Code: "399006", Name: "创业板指", FetchCode: "SZ399006"},
		{Code: "000688", Name: "科创50", FetchCode: "SH000688"},
		{Code: "899050", Name: "北证50", FetchCode: "BJ899050"},
		{Code: "000905", Name: "中证500", FetchCode: "SH000905"},
		{Code: "000852", Name: "中证1000", FetchCode: "SH000852"},
		{Code: "000300", Name: "沪深300", FetchCode: "SH000300"},
	}

	for _, idx := range indexes {
		result, err := p.registry.Execute(ctx, tool.ToolCall{
			Name: "eastmoney",
			Arguments: map[string]interface{}{
				"action": "get_stock_quote",
				"code":   idx.FetchCode,
			},
		})
		if err != nil {
			continue
		}

		if quote, ok := result.Output.(tool.StockQuote); ok {
			marketData.IndexQuotes = append(marketData.IndexQuotes, IndexQuote{
				Code:       idx.Code,
				Name:       idx.Name,
				Price:      quote.Price,
				Change:     quote.Change,
				ChangeRate: quote.ChangeRate,
				Amount:     quote.Amount,
			})
		}
	}

	// 获取市场统计数据
	result, err := p.registry.Execute(ctx, tool.ToolCall{
		Name: "eastmoney",
		Arguments: map[string]interface{}{
			"action":    "get_stock_list",
			"page":      1,
			"page_size": 5000,
		},
	})
	if err == nil {
		if items, ok := result.Output.([]tool.MarketItem); ok {
			upCount, downCount, limitUp, limitDown := 0, 0, 0, 0
			for _, item := range items {
				if item.ChangeRate > 0 {
					upCount++
				} else if item.ChangeRate < 0 {
					downCount++
				}
				if item.ChangeRate >= 9.9 {
					limitUp++
				} else if item.ChangeRate <= -9.9 {
					limitDown++
				}
			}
			marketData.UpCount = upCount
			marketData.DownCount = downCount
			marketData.LimitUp = limitUp
			marketData.LimitDown = limitDown
		}
	}

	fillMarketSentiment(&marketData)
	return marketData, nil
}

// GetRecentNews 获取近N天重大新闻
func (p *DefaultDataProvider) GetRecentNews(ctx context.Context, days int, limit int) ([]NewsEvent, error) {
	if p == nil || p.newsRepo == nil {
		return nil, errors.New("provider_not_initialized")
	}

	now := time.Now().In(p.location)
	dateFrom := now.AddDate(0, 0, -days)

	// 不使用 MinPriority 过滤，因为 json_extract 可能在某些记录上失败
	events, _, err := p.newsRepo.ListEvents(ctx, news.EventQuery{
		Page:     1,
		PageSize: limit,
		DateFrom: &dateFrom,
		DateTo:   &now,
		SortBy:   "published_at",
	})
	if err != nil {
		return nil, err
	}

	// 转换为简化的NewsEvent
	result := make([]NewsEvent, 0, len(events))
	for _, e := range events {
		ne := NewsEvent{
			Title:       e.Title,
			Summary:     e.Summary,
			Domains:     make([]string, 0, len(e.Domains)),
			Priority:    3,
			PublishedAt: e.PublishedAt,
		}

		// 转换domains
		for _, d := range e.Domains {
			ne.Domains = append(ne.Domains, string(d))
		}

		// 提取优先级和方向
		if e.FunnelResult != nil {
			ne.Priority = e.FunnelResult.L2Priority
			if len(e.FunnelResult.L2AffectedAssets) > 0 {
				ne.Direction = string(e.FunnelResult.L2AffectedAssets[0].Direction)
			}
		}

		// 提取情绪方向
		if ne.Direction == "" && e.Labels.Sentiment != 0 {
			if e.Labels.Sentiment > 0.3 {
				ne.Direction = "bullish"
			} else if e.Labels.Sentiment < -0.3 {
				ne.Direction = "bearish"
			} else {
				ne.Direction = "mixed"
			}
		}

		result = append(result, ne)
	}

	return result, nil
}

// GetHoldings 获取用户持仓
func (p *DefaultDataProvider) GetHoldings(ctx context.Context, accountID int64) ([]Position, error) {
	if p == nil || p.investRepo == nil {
		return nil, errors.New("provider_not_initialized")
	}

	positions, err := p.investRepo.ListPositions(ctx, accountID)
	if err != nil {
		return nil, err
	}

	result := make([]Position, 0, len(positions))
	for _, pos := range positions {
		value := pos.Quantity * pos.AvgCost
		if pos.MarketPrice != nil {
			value = pos.Quantity * (*pos.MarketPrice)
		}
		result = append(result, Position{
			Symbol:   pos.Instrument.Symbol,
			Name:     pos.Instrument.Name,
			Industry: pos.Instrument.Industry,
			Quantity: pos.Quantity,
			Value:    value,
		})
	}

	return result, nil
}

// GetStockList 获取全市场股票列表
func (p *DefaultDataProvider) GetStockList(ctx context.Context) ([]tool.MarketItem, error) {
	if p == nil || p.registry == nil {
		return nil, errors.New("provider_not_initialized")
	}

	result, err := p.registry.Execute(ctx, tool.ToolCall{
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

// GetFinancialData 获取股票财务数据
func (p *DefaultDataProvider) GetFinancialData(ctx context.Context, symbol string) (map[string]interface{}, error) {
	if p == nil || p.financialSvc == nil {
		return nil, errors.New("provider_not_initialized")
	}

	return p.financialSvc.GetFinancialData(ctx, symbol)
}

func unwrapDailyReviewPayload(raw map[string]interface{}) map[string]interface{} {
	if data, ok := raw["data"].(map[string]interface{}); ok {
		return data
	}
	return raw
}

func canonicalMarketIndexName(name string) string {
	trimmed := strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	if canonical, ok := marketIndexAliases[trimmed]; ok {
		return canonical
	}
	return ""
}

func extractDailyReviewIndexes(payload map[string]interface{}) []IndexQuote {
	indexes, ok := payload["indexes"].([]interface{})
	if !ok {
		return nil
	}
	byName := map[string]IndexQuote{}
	for _, idx := range indexes {
		idxMap, ok := idx.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := idxMap["name"].(string)
		canonicalName := canonicalMarketIndexName(name)
		if canonicalName == "" {
			continue
		}

		quote := IndexQuote{Name: canonicalName}
		if code, ok := idxMap["code"].(string); ok {
			quote.Code = code
		}
		if price, ok := toFloat64(idxMap["price"]); ok {
			quote.Price = price
		}
		if change, ok := toFloat64(idxMap["change"]); ok {
			quote.Change = change
		}
		if changeRate, ok := toFloat64(idxMap["change_rate"]); ok {
			quote.ChangeRate = changeRate
		}
		if amount, ok := toFloat64(idxMap["amount"]); ok {
			quote.Amount = amount
		}
		byName[canonicalName] = quote
	}

	return orderMarketIndexes(byName)
}

func extractFupanIndexQuotes(payload map[string]interface{}) []IndexQuote {
	fupan, ok := payload["fupan_report"].(map[string]interface{})
	if !ok {
		return nil
	}
	rawIndexes, ok := fupan["indexes"].([]interface{})
	if !ok {
		return nil
	}

	byName := map[string]IndexQuote{}
	for _, raw := range rawIndexes {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := item["name"].(string)
		canonicalName := canonicalMarketIndexName(name)
		if canonicalName == "" {
			continue
		}
		price, okPrice := toFloat64(item["price"])
		change, okChange := toFloat64(item["change"])
		changeRate, okRate := toFloat64(item["change_rate"])
		if !okPrice {
			price, okPrice = parseLooseFloat(item["price"])
		}
		if !okChange {
			change, okChange = parseLooseFloat(item["change"])
		}
		if !okRate {
			changeRate, okRate = parseLooseFloat(item["change_rate"])
		}
		if !okPrice || !okChange || !okRate {
			continue
		}
		byName[canonicalName] = IndexQuote{
			Code:       canonicalName,
			Name:       canonicalName,
			Price:      price,
			Change:     change,
			ChangeRate: changeRate,
		}
	}

	return orderMarketIndexes(byName)
}

func orderMarketIndexes(byName map[string]IndexQuote) []IndexQuote {
	if len(byName) == 0 {
		return nil
	}
	result := make([]IndexQuote, 0, len(byName))
	for _, name := range canonicalMarketIndexes {
		if quote, ok := byName[name]; ok {
			result = append(result, quote)
		}
	}
	return result
}

func extractFupanRiseDown(payload map[string]interface{}) (int, int, bool) {
	fupan, ok := payload["fupan_report"].(map[string]interface{})
	if !ok {
		return 0, 0, false
	}
	up, upOK := toInt(fupan["up_count"])
	down, downOK := toInt(fupan["down_count"])
	if upOK && downOK && isReasonableMarketBreadth(up, down) {
		return up, down, true
	}
	if upOK && downOK {
		log.Printf("[选股数据] 忽略异常fupan_report计数 up=%d down=%d", up, down)
	}

	if breadthText, ok := fupan["breadth_text"].(string); ok {
		if up, down, ok := parseRiseDownFromText(breadthText); ok {
			return up, down, true
		}
	}
	if summary, ok := fupan["summary"].(string); ok {
		if up, down, ok := parseRiseDownFromText(summary); ok {
			return up, down, true
		}
	}
	return 0, 0, false
}

func parseRiseDownFromText(text string) (int, int, bool) {
	matches := riseFallPattern.FindStringSubmatch(text)
	if len(matches) == 4 {
		up, upOK := toInt(matches[1])
		down, downOK := toInt(matches[3])
		if !upOK || !downOK {
			return 0, 0, false
		}
		if !isReasonableMarketBreadth(up, down) {
			return 0, 0, false
		}
		return up, down, true
	}

	up, upOK := parseCountFromText(text, riseCountPattern)
	down, downOK := parseCountFromText(text, downCountPattern)
	if !upOK || !downOK {
		return 0, 0, false
	}
	if !isReasonableMarketBreadth(up, down) {
		return 0, 0, false
	}
	return up, down, true
}

func parseCountFromText(text string, pattern *regexp.Regexp) (int, bool) {
	if pattern == nil {
		return 0, false
	}
	m := pattern.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0, false
	}
	raw := strings.TrimSpace(m[1])
	raw = strings.ReplaceAll(raw, ",", "")
	raw = strings.ReplaceAll(raw, "，", "")
	return toInt(raw)
}

func isReasonableMarketBreadth(up int, down int) bool {
	return up+down >= 500
}

func fillMarketSentiment(marketData *MarketData) {
	if marketData == nil {
		return
	}
	total := marketData.UpCount + marketData.DownCount
	if total <= 0 {
		return
	}
	upRatio := float64(marketData.UpCount) / float64(total)
	switch {
	case upRatio > 0.7:
		marketData.MarketSentiment = "very_bullish"
	case upRatio > 0.55:
		marketData.MarketSentiment = "bullish"
	case upRatio > 0.45:
		marketData.MarketSentiment = "neutral"
	case upRatio > 0.3:
		marketData.MarketSentiment = "bearish"
	default:
		marketData.MarketSentiment = "very_bearish"
	}
}

func parseLooseFloat(v interface{}) (float64, bool) {
	s, ok := v.(string)
	if !ok {
		return 0, false
	}
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "＋", "+")
	s = strings.ReplaceAll(s, "－", "-")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, false
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0, false
	}
	return f, true
}

// 辅助函数：将interface{}转换为float64
func toFloat64(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err == nil
	default:
		return 0, false
	}
}

// 辅助函数：将interface{}转换为int
func toInt(v interface{}) (int, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case int32:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	case json.Number:
		f, err := val.Float64()
		return int(f), err == nil
	case string:
		var i int
		_, err := fmt.Sscanf(val, "%d", &i)
		return i, err == nil
	default:
		return 0, false
	}
}
