package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/investment"
	"genFu/internal/tool"
)

type DailyReviewService struct {
	model      model.ToolCallingChatModel
	registry   *tool.Registry
	repo       *Repository
	investRepo *investment.Repository
	accountID  int64
	location   *time.Location
}

func NewDailyReviewService(model model.ToolCallingChatModel, registry *tool.Registry, repo *Repository, investRepo *investment.Repository) *DailyReviewService {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	return &DailyReviewService{
		model:      model,
		registry:   registry,
		repo:       repo,
		investRepo: investRepo,
		accountID:  1,
		location:   loc,
	}
}

func (s *DailyReviewService) Run(ctx context.Context) (int64, error) {
	if s == nil || s.model == nil || s.repo == nil {
		return 0, errors.New("daily_review_service_not_initialized")
	}
	now := time.Now().In(s.location)
	if ctx.Value("now") != nil { //for test
		if t, ok := ctx.Value("now").(time.Time); ok {
			now = t.In(s.location)
		}
	}
	return s.RunWithDate(ctx, now)
}

func (s *DailyReviewService) RunWithDate(ctx context.Context, now time.Time) (int64, error) {
	if s == nil || s.model == nil || s.repo == nil {
		return 0, errors.New("daily_review_service_not_initialized")
	}
	payload, err := s.buildData(ctx, now)
	if err != nil {
		return 0, err
	}
	warning := ""
	if rawWarning, ok := payload["critical_warning"].(string); ok {
		warning = strings.TrimSpace(rawWarning)
	}
	date := now.Format("2006-01-02")
	userPayload := map[string]interface{}{
		"date": date,
		"data": payload,
	}
	raw, _ := json.Marshal(userPayload)
	userPrompt := "请根据以下数据生成每日收盘复盘总结，严格按模板输出：\n" + string(raw)
	resp, err := s.model.Generate(ctx, []*schema.Message{
		schema.SystemMessage(dailyReviewPrompt),
		schema.UserMessage(userPrompt),
	})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, errors.New("empty_llm_response")
	}
	summary := resp.Content
	if warning != "" {
		prefix := "【重要提示】" + warning
		if !strings.HasPrefix(strings.TrimSpace(summary), "【重要提示】") {
			summary = prefix + "\n" + strings.TrimSpace(summary)
		}
	}
	if report, ok := payload["fupan_report"].(*FupanReport); ok {
		if compare, err := compareFupanSummary(summary, report); err == nil {
			userPayload["fupan_report_compare"] = compare
		}
	}
	return s.repo.CreateDailyReviewReport(ctx, date, userPayload, summary)
}

func (s *DailyReviewService) buildData(ctx context.Context, now time.Time) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	date := now.Format("2006-01-02")
	result["date"] = date

	indexes := []map[string]string{
		{"code": "000001", "name": "上证指数", "fetch_code": "SH000001"},
		{"code": "399001", "name": "深证成指", "fetch_code": "SZ399001"},
		{"code": "399006", "name": "创业板指", "fetch_code": "SZ399006"},
		{"code": "000688", "name": "科创50", "fetch_code": "SH000688"},
		{"code": "899050", "name": "北证50", "fetch_code": "BJ899050"},
		{"code": "000905", "name": "中证500", "fetch_code": "SH000905"},
		{"code": "000852", "name": "中证1000", "fetch_code": "SH000852"},
		{"code": "000300", "name": "沪深300", "fetch_code": "SH000300"},
	}
	indexQuotes := []map[string]interface{}{}
	indexErrors := 0
	for _, idx := range indexes {
		q, err := s.fetchStockQuote(ctx, idx["fetch_code"])
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
	if indexErrors < len(indexes) {
		result["indexes"] = indexQuotes
	}

	stockList, stockListErr := s.fetchStockList(ctx, 1, 5000)
	if stockListErr != nil {
		result["stock_list_error"] = stockListErr.Error()
	} else if len(stockList) == 0 {
		result["stock_list_error"] = "empty_stock_list"
	} else {
		metrics := s.computeMarketMetrics(stockList)
		result["market_metrics"] = metrics
		result["top_amount"] = metrics["top_amount"]
		result["top_amplitude"] = metrics["top_amplitude"]
		result["top_change"] = metrics["top_change"]
	}

	positions, summary := s.loadHoldings(ctx)
	result["holdings"] = positions
	result["portfolio_summary"] = summary

	hasIndexes := indexErrors < len(indexes)
	hasMarketMetrics := stockListErr == nil && len(stockList) > 0
	marketFailure := !hasIndexes && !hasMarketMetrics
	if marketFailure {
		result["critical_warning"] = "大盘指数及板块数据获取失败，报告未必可信"
		delete(result, "indexes")
		delete(result, "market_metrics")
		delete(result, "top_amount")
		delete(result, "top_amplitude")
		delete(result, "top_change")
	}

	skipFupan, _ := ctx.Value("skip_fupan").(bool)
	if !skipFupan {
		fupanReport, err := s.fetchFupanReport(ctx, s.fupanDate(now))
		if err != nil {
			result["fupan_report_error"] = err.Error()
		} else {
			result["fupan_report"] = fupanReport
		}
	}

	return result, nil
}

func isTradingDay(now time.Time) bool {
	switch now.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	default:
		return true
	}
}

func (s *DailyReviewService) fetchStockQuote(ctx context.Context, code string) (*tool.StockQuote, error) {
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

func (s *DailyReviewService) fetchStockList(ctx context.Context, page, pageSize int) ([]tool.MarketItem, error) {
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

func (s *DailyReviewService) loadHoldings(ctx context.Context) ([]map[string]interface{}, map[string]interface{}) {
	positions := []map[string]interface{}{}
	summary := map[string]interface{}{}
	if s.investRepo == nil {
		return positions, summary
	}
	list, err := s.investRepo.ListPositions(ctx, s.accountID)
	if err == nil {
		for _, p := range list {
			marketPrice := p.MarketPrice
			price := p.AvgCost
			if marketPrice != nil {
				price = *marketPrice
			}
			positions = append(positions, map[string]interface{}{
				"symbol":       p.Instrument.Symbol,
				"name":         p.Instrument.Name,
				"asset_type":   p.Instrument.AssetType,
				"quantity":     p.Quantity,
				"avg_cost":     p.AvgCost,
				"market_price": marketPrice,
				"value":        price * p.Quantity,
			})
		}
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
	return positions, summary
}

func (s *DailyReviewService) computeMarketMetrics(items []tool.MarketItem) map[string]interface{} {
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

func topByAmount(items []tool.MarketItem, n int) []tool.MarketItem {
	cp := append([]tool.MarketItem{}, items...)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Amount > cp[j].Amount
	})
	if n > len(cp) {
		n = len(cp)
	}
	return cp[:n]
}

func topByAmplitude(items []tool.MarketItem, n int) []tool.MarketItem {
	cp := append([]tool.MarketItem{}, items...)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Amplitude > cp[j].Amplitude
	})
	if n > len(cp) {
		n = len(cp)
	}
	return cp[:n]
}

func topByChange(items []tool.MarketItem, n int) []tool.MarketItem {
	cp := append([]tool.MarketItem{}, items...)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].ChangeRate > cp[j].ChangeRate
	})
	if n > len(cp) {
		n = len(cp)
	}
	return cp[:n]
}

func (s *DailyReviewService) fupanDate(now time.Time) string {
	reportTime := now
	if now.Hour() < 15 {
		reportTime = now.AddDate(0, 0, -1)
	}
	for reportTime.Weekday() == time.Saturday || reportTime.Weekday() == time.Sunday {
		reportTime = reportTime.AddDate(0, 0, -1)
	}
	return reportTime.Format("20060102")
}

var dailyReviewPrompt = `你是专业投资者的复盘助手，请基于输入数据生成每日收盘复盘总结。\n\n输出结构必须包含以下 7 个板块并使用清晰分段：\n1. 大盘总览\n2. 板块热力图\n3. 个股风云榜\n4. 资金流向\n5. 持仓标的跟踪（个人）\n6. 情绪面 & 市场温度\n7. 明日策略展望\n\n如果输入包含 critical_warning，必须作为首句并以“【重要提示】”开头。请根据可用数据进行总结，缺失的数据标注“数据暂缺”，不要编造。输出使用自然中文，不要添加额外说明或 Markdown。`
