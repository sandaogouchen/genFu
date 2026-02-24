package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/investment"
	"genFu/internal/rsshub"
	"genFu/internal/tool"
)

type NextOpenGuideService struct {
	model      model.ToolCallingChatModel
	registry   *tool.Registry
	repo       *Repository
	investRepo *investment.Repository
	accountID  int64
	location   *time.Location
	newsRoutes []string
	newsLimit  int
}

type NextOpenGuideOutput struct {
	Brief string `json:"brief"`
	Guide string `json:"guide"`
}

type GuideHolding struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name"`
	Quantity    float64 `json:"quantity"`
	AvgCost     float64 `json:"avg_cost"`
	MarketPrice float64 `json:"market_price"`
	Value       float64 `json:"value"`
	Ratio       float64 `json:"ratio"`
}

type GuideHoldings struct {
	Positions  []GuideHolding `json:"positions"`
	TotalValue float64        `json:"total_value"`
}

type GuideHoldingMarket struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	ChangeRate float64 `json:"change_rate"`
	Error      string  `json:"error,omitempty"`
}

func NewNextOpenGuideService(model model.ToolCallingChatModel, registry *tool.Registry, repo *Repository, investRepo *investment.Repository, accountID int64, routes []string, newsLimit int) *NextOpenGuideService {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	if accountID == 0 {
		accountID = 1
	}
	if newsLimit <= 0 {
		newsLimit = 10
	}
	return &NextOpenGuideService{
		model:      model,
		registry:   registry,
		repo:       repo,
		investRepo: investRepo,
		accountID:  accountID,
		location:   loc,
		newsRoutes: routes,
		newsLimit:  newsLimit,
	}
}

func (s *NextOpenGuideService) Run(ctx context.Context) (int64, error) {
	if s == nil || s.model == nil || s.repo == nil {
		return 0, errors.New("next_open_service_not_initialized")
	}
	payload, err := s.buildData(ctx)
	if err != nil {
		return 0, err
	}
	warning := ""
	if rawWarning, ok := payload["critical_warning"].(string); ok {
		warning = strings.TrimSpace(rawWarning)
	}
	now := time.Now().In(s.location)
	date := now.Format("2006-01-02")
	userPayload := map[string]interface{}{
		"date": date,
		"data": payload,
	}
	raw, _ := json.Marshal(userPayload)
	userPrompt := "请根据以下数据生成次日开盘指导，严格输出JSON：{\"brief\":\"...\",\"guide\":\"...\"}\n" + string(raw)
	resp, err := s.model.Generate(ctx, []*schema.Message{
		schema.SystemMessage(nextOpenGuidePrompt),
		schema.UserMessage(userPrompt),
	})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, errors.New("empty_llm_response")
	}
	output, ok := parseNextOpenGuideOutput(resp.Content)
	if !ok {
		output = NextOpenGuideOutput{Guide: strings.TrimSpace(resp.Content)}
	}
	if warning != "" {
		prefix := "【重要提示】" + warning
		if strings.TrimSpace(output.Brief) == "" {
			output.Brief = prefix
		} else if !strings.HasPrefix(strings.TrimSpace(output.Brief), "【重要提示】") {
			output.Brief = prefix + "\n" + strings.TrimSpace(output.Brief)
		}
	}
	outputPayload := map[string]interface{}{
		"brief": output.Brief,
		"guide": output.Guide,
	}
	userPayload["output"] = outputPayload
	summaryRaw, _ := json.Marshal(outputPayload)
	return s.repo.CreateNextOpenGuideReport(ctx, date, userPayload, string(summaryRaw))
}

func (s *NextOpenGuideService) buildData(ctx context.Context) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	holdings, holdingsMarket, summary, err := s.loadHoldings(ctx)
	if err != nil {
		result["holdings_error"] = err.Error()
	}
	result["holdings"] = holdings
	result["holdings_market"] = holdingsMarket
	result["portfolio_summary"] = summary

	indexes := []map[string]string{
		{"code": "000001", "name": "上证指数"},
		{"code": "399001", "name": "深证成指"},
		{"code": "399006", "name": "创业板指"},
		{"code": "000688", "name": "科创50"},
		{"code": "899050", "name": "北证50"},
		{"code": "000905", "name": "中证500"},
		{"code": "000852", "name": "中证1000"},
		{"code": "000300", "name": "沪深300"},
	}
	indexQuotes := []map[string]interface{}{}
	indexErrors := 0
	for _, idx := range indexes {
		q, err := s.fetchStockQuote(ctx, idx["code"])
		if err != nil {
			indexErrors++
			indexQuotes = append(indexQuotes, map[string]interface{}{
				"code":  idx["code"],
				"name":  idx["name"],
				"error": err.Error(),
			})
			continue
		}
		indexQuotes = append(indexQuotes, map[string]interface{}{
			"code":        idx["code"],
			"name":        idx["name"],
			"price":       q.Price,
			"change":      q.Change,
			"change_rate": q.ChangeRate,
			"amount":      q.Amount,
		})
	}
	hasIndexes := indexErrors < len(indexes)
	if hasIndexes {
		result["indexes"] = indexQuotes
	}

	hasMarketMetrics := false
	marketItems, err := s.fetchStockList(ctx, 1, 800)
	if err != nil {
		result["market_metrics_error"] = err.Error()
	} else if len(marketItems) == 0 {
		result["market_metrics_error"] = "empty_market_list"
	} else {
		metrics := computeGuideMarketMetrics(marketItems)
		result["market_metrics"] = metrics
		result["top_amount"] = metrics["top_amount"]
		result["top_amplitude"] = metrics["top_amplitude"]
		result["top_change"] = metrics["top_change"]
		hasMarketMetrics = true
	}
	if !hasIndexes && !hasMarketMetrics {
		result["critical_warning"] = "大盘指数及板块数据获取失败，报告未必可信"
	}

	newsItems, err := s.fetchNews(ctx)
	if err != nil {
		result["news_error"] = err.Error()
	} else {
		news := compactGuideNews(newsItems, s.newsLimit)
		if len(news) > 0 {
			result["news"] = news
		}
	}
	return result, nil
}

func (s *NextOpenGuideService) loadHoldings(ctx context.Context) (GuideHoldings, []GuideHoldingMarket, map[string]interface{}, error) {
	holdings := GuideHoldings{Positions: []GuideHolding{}}
	market := []GuideHoldingMarket{}
	summary := map[string]interface{}{}
	if s.investRepo == nil || s.accountID == 0 {
		return holdings, market, summary, nil
	}
	positions, err := s.investRepo.ListPositions(ctx, s.accountID)
	if err != nil {
		return holdings, market, summary, err
	}
	var total float64
	for _, p := range positions {
		price := p.AvgCost
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			price = *p.MarketPrice
		}
		value := price * p.Quantity
		total += value
		holdings.Positions = append(holdings.Positions, GuideHolding{
			Symbol:      p.Instrument.Symbol,
			Name:        p.Instrument.Name,
			Quantity:    p.Quantity,
			AvgCost:     p.AvgCost,
			MarketPrice: price,
			Value:       value,
		})
	}
	holdings.TotalValue = total
	if total > 0 {
		for i := range holdings.Positions {
			holdings.Positions[i].Ratio = holdings.Positions[i].Value / total
		}
	}
	for _, position := range holdings.Positions {
		market = append(market, s.fetchQuote(ctx, position.Symbol, position.Name))
	}
	if sum, err := s.investRepo.GetPortfolioSummary(ctx, s.accountID); err == nil {
		summary = map[string]interface{}{
			"position_count": sum.PositionCount,
			"trade_count":    sum.TradeCount,
			"total_value":    sum.TotalValue,
			"total_cost":     sum.TotalCost,
			"total_pnl":      sum.TotalPnL,
			"valuation_at":   sum.ValuationAt,
		}
	}
	return holdings, market, summary, nil
}

func (s *NextOpenGuideService) fetchQuote(ctx context.Context, symbol string, name string) GuideHoldingMarket {
	if s.registry == nil || strings.TrimSpace(symbol) == "" {
		return GuideHoldingMarket{Symbol: symbol, Name: name, Error: "tool_registry_not_initialized"}
	}
	call := tool.ToolCall{
		Name: "eastmoney",
		Arguments: map[string]interface{}{
			"action": "get_stock_quote",
			"code":   symbol,
		},
	}
	result, err := s.registry.Execute(ctx, call)
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	move := GuideHoldingMarket{Symbol: symbol, Name: name, Error: result.Error}
	if result.Output != nil {
		if quote, ok := result.Output.(tool.StockQuote); ok {
			move.Symbol = quote.Code
			move.Name = quote.Name
			move.Price = quote.Price
			move.Change = quote.Change
			move.ChangeRate = quote.ChangeRate
			return move
		}
		raw, _ := json.Marshal(result.Output)
		_ = json.Unmarshal(raw, &move)
		if move.Symbol == "" {
			move.Symbol = symbol
		}
		if move.Name == "" {
			move.Name = name
		}
	}
	return move
}

func (s *NextOpenGuideService) fetchStockQuote(ctx context.Context, code string) (*tool.StockQuote, error) {
	if s.registry == nil {
		return nil, errors.New("tool_registry_not_initialized")
	}
	call := tool.ToolCall{Name: "eastmoney", Arguments: map[string]interface{}{"action": "get_stock_quote", "code": code}}
	result, err := s.registry.Execute(ctx, call)
	if err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	if q, ok := result.Output.(*tool.StockQuote); ok {
		return q, nil
	}
	if q, ok := result.Output.(tool.StockQuote); ok {
		return &q, nil
	}
	return nil, errors.New("unexpected_quote_type")
}

func (s *NextOpenGuideService) fetchStockList(ctx context.Context, page int, pageSize int) ([]tool.MarketItem, error) {
	if s.registry == nil {
		return nil, errors.New("tool_registry_not_initialized")
	}
	call := tool.ToolCall{Name: "eastmoney", Arguments: map[string]interface{}{"action": "get_stock_list", "page": page, "page_size": pageSize}}
	result, err := s.registry.Execute(ctx, call)
	if err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	if list, ok := result.Output.([]tool.MarketItem); ok {
		return list, nil
	}
	if list, ok := result.Output.([]interface{}); ok {
		items := make([]tool.MarketItem, 0, len(list))
		for _, item := range list {
			if m, ok := item.(tool.MarketItem); ok {
				items = append(items, m)
			}
		}
		return items, nil
	}
	return nil, errors.New("unexpected_stock_list_type")
}

func (s *NextOpenGuideService) fetchNews(ctx context.Context) ([]rsshub.Item, error) {
	if s.registry == nil {
		return nil, errors.New("tool_registry_not_initialized")
	}
	items := make([]rsshub.Item, 0)
	for _, route := range s.newsRoutes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}
		call := tool.ToolCall{
			Name: "rsshub",
			Arguments: map[string]interface{}{
				"action": "fetch_feed",
				"route":  route,
				"limit":  s.newsLimit,
			},
		}
		result, err := s.registry.Execute(ctx, call)
		if err != nil {
			continue
		}
		if out, ok := result.Output.([]rsshub.Item); ok {
			items = append(items, out...)
			continue
		}
		raw, _ := json.Marshal(result.Output)
		var parsed []rsshub.Item
		if err := json.Unmarshal(raw, &parsed); err == nil {
			items = append(items, parsed...)
		}
	}
	return items, nil
}

func computeGuideMarketMetrics(items []tool.MarketItem) map[string]interface{} {
	totalAmount := 0.0
	upCount := 0
	downCount := 0
	flatCount := 0
	limitUp := 0
	limitDown := 0

	for _, item := range items {
		totalAmount += item.Amount
		if item.ChangeRate > 0 {
			upCount++
		} else if item.ChangeRate < 0 {
			downCount++
		} else {
			flatCount++
		}
		if item.ChangeRate >= 9.9 {
			limitUp++
		}
		if item.ChangeRate <= -9.9 {
			limitDown++
		}
	}
	topAmount := topByAmount(items, 20)
	topAmplitude := topByAmplitude(items, 20)
	topChange := topByChange(items, 20)
	return map[string]interface{}{
		"total_amount":  totalAmount,
		"up":            upCount,
		"down":          downCount,
		"flat":          flatCount,
		"limit_up":      limitUp,
		"limit_down":    limitDown,
		"top_amount":    topAmount,
		"top_amplitude": topAmplitude,
		"top_change":    topChange,
	}
}

func compactGuideNews(items []rsshub.Item, limit int) []map[string]interface{} {
	if limit <= 0 {
		limit = 10
	}
	if len(items) > limit {
		items = items[:limit]
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{
			"title":        strings.TrimSpace(item.Title),
			"link":         strings.TrimSpace(item.Link),
			"published_at": item.PublishedAt,
			"description":  trimText(item.Description, 200),
		})
	}
	return result
}

func trimText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func parseNextOpenGuideOutput(content string) (NextOpenGuideOutput, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return NextOpenGuideOutput{}, false
	}
	tryParse := func(raw string) (NextOpenGuideOutput, bool) {
		var parsed NextOpenGuideOutput
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return NextOpenGuideOutput{}, false
		}
		if strings.TrimSpace(parsed.Brief) == "" && strings.TrimSpace(parsed.Guide) == "" {
			return NextOpenGuideOutput{}, false
		}
		return parsed, true
	}
	if parsed, ok := tryParse(content); ok {
		return parsed, true
	}
	if idx := strings.Index(content, "```"); idx >= 0 {
		end := strings.LastIndex(content, "```")
		if end > idx {
			raw := strings.TrimSpace(content[idx+3 : end])
			raw = strings.TrimPrefix(raw, "json")
			raw = strings.TrimSpace(raw)
			if parsed, ok := tryParse(raw); ok {
				return parsed, true
			}
		}
	}
	return NextOpenGuideOutput{}, false
}

var nextOpenGuidePrompt = "你是专业交易顾问，请基于提供的数据输出次日开盘指导。要求：1) 输出严格JSON，格式：{\"brief\":\"...\",\"guide\":\"...\"}。2) 如果输入包含 critical_warning，必须作为 brief 首句并以“【重要提示】”开头。3) brief 用5-8句中文概括市场、持仓、板块轮动与调仓判断，缺失数据写“数据暂缺”。4) guide 以文字列出买卖/持有/观望建议，包含标的、方向、理由与风险提示。5) 只基于输入数据，不要编造，不要输出Markdown。"
