package tool

import (
	"context"
	"fmt"
	"genFu/internal/tushare"
	"log"
	"sort"
)

// TushareTool provides Tushare Pro data access as a registered tool.
type TushareTool struct {
	client *tushare.Client
}

// NewTushareTool creates a new TushareTool. Returns nil if client is nil.
func NewTushareTool(client *tushare.Client) *TushareTool {
	if client == nil {
		return nil
	}
	return &TushareTool{client: client}
}

func (t *TushareTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "tushare",
		Description: `Tushare Pro 金融数据工具。提供 A 股行情（日/周/月线，支持前后复权）、股票基本信息、交易日历、每日基本面指标（PE/PB/换手率/市值）、财务三表（利润表/资产负债表/现金流量表）、财务指标、指数成分权重、分红送股等数据。
支持的 action：
  P0: get_daily, get_weekly, get_monthly, get_stock_basic, get_trade_cal, get_daily_basic
  P1: get_income, get_balance, get_cashflow, get_fina_indicator, get_index_weight, get_dividend`,
		Params: map[string]string{
			"action":      "(必填) 要执行的操作",
			"ts_code":     "(可选) 股票代码，支持 000001/000001.SZ/SZ000001 等格式",
			"start_date":  "(可选) 开始日期 YYYYMMDD",
			"end_date":    "(可选) 结束日期 YYYYMMDD",
			"trade_date":  "(可选) 交易日期 YYYYMMDD",
			"adj":         "(可选) 复权类型: qfq=前复权, hfq=后复权, 空=不复权",
			"list_status": "(可选) 上市状态: L=上市, D=退市, P=暂停",
			"exchange":    "(可选) 交易所: SSE=上交所, SZSE=深交所",
			"period":      "(可选) 报告期 YYYYMMDD，如 20231231",
			"index_code":  "(可选) 指数代码，如 399300.SZ",
		},
		Required: []string{"action"},
	}
}

func (t *TushareTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	action, err := requireString(args, "action")
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}

	switch action {
	case "get_daily":
		return t.getDailyKline(ctx, args, "daily")
	case "get_weekly":
		return t.getDailyKline(ctx, args, "weekly")
	case "get_monthly":
		return t.getDailyKline(ctx, args, "monthly")
	case "get_stock_basic":
		return t.getStockBasic(ctx, args)
	case "get_trade_cal":
		return t.getTradeCal(ctx, args)
	case "get_daily_basic":
		return t.getDailyBasic(ctx, args)
	case "get_income":
		return t.getIncome(ctx, args)
	case "get_balance":
		return t.getBalance(ctx, args)
	case "get_cashflow":
		return t.getCashflow(ctx, args)
	case "get_fina_indicator":
		return t.getFinaIndicator(ctx, args)
	case "get_index_weight":
		return t.getIndexWeight(ctx, args)
	case "get_dividend":
		return t.getDividend(ctx, args)
	default:
		return ToolResult{Name: "tushare", Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// getDailyKline handles get_daily, get_weekly, get_monthly with optional adj.
func (t *TushareTool) getDailyKline(ctx context.Context, args map[string]interface{}, period string) (ToolResult, error) {
	tsCode, err := requireString(args, "ts_code")
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	tsCode = tushare.NormalizeTsCode(tsCode)

	startDate := optionalString(args, "start_date")
	endDate := optionalString(args, "end_date")
	adj := optionalString(args, "adj")

	var bars []tushare.DailyBar
	switch period {
	case "weekly":
		bars, err = t.client.Weekly(ctx, tsCode, startDate, endDate)
	case "monthly":
		bars, err = t.client.Monthly(ctx, tsCode, startDate, endDate)
	default:
		bars, err = t.client.Daily(ctx, tsCode, startDate, endDate)
	}
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}

	// Apply adjustment if requested
	if adj == "qfq" || adj == "hfq" {
		adjusted, adjErr := t.applyAdj(ctx, bars, tsCode, startDate, endDate, adj)
		if adjErr != nil {
			log.Printf("tushare: 复权计算失败，返回不复权数据: %v", adjErr)
		} else {
			bars = adjusted
		}
	}

	// Sort by date ascending
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].TradeDate < bars[j].TradeDate
	})

	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"ts_code":     tsCode,
		"period":      period,
		"adj":         adj,
		"count":       len(bars),
		"data_source": "tushare",
		"bars":        bars,
	}}, nil
}

// applyAdj applies forward or backward adjustment to daily bars.
func (t *TushareTool) applyAdj(ctx context.Context, bars []tushare.DailyBar, tsCode, startDate, endDate, adj string) ([]tushare.DailyBar, error) {
	if len(bars) == 0 {
		return bars, nil
	}

	factors, err := t.client.AdjFactor(ctx, tsCode, startDate, endDate)
	if err != nil {
		return nil, err
	}

	factorMap := make(map[string]float64, len(factors))
	for _, f := range factors {
		factorMap[f.TradeDate] = f.AdjFactor
	}

	// Find the latest factor for qfq
	var latestFactor float64
	for _, bar := range bars {
		if f, ok := factorMap[bar.TradeDate]; ok && f > latestFactor {
			latestFactor = f
		}
	}
	if latestFactor == 0 {
		latestFactor = 1
	}

	// Find the earliest factor for hfq
	var earliestFactor float64
	for _, bar := range bars {
		if f, ok := factorMap[bar.TradeDate]; ok {
			if earliestFactor == 0 || f < earliestFactor {
				earliestFactor = f
			}
		}
	}
	if earliestFactor == 0 {
		earliestFactor = 1
	}

	adjusted := make([]tushare.DailyBar, len(bars))
	copy(adjusted, bars)

	for i, bar := range adjusted {
		factor, ok := factorMap[bar.TradeDate]
		if !ok {
			continue
		}
		var ratio float64
		if adj == "qfq" {
			ratio = factor / latestFactor
		} else {
			ratio = factor / earliestFactor
		}
		adjusted[i].Open = bar.Open * ratio
		adjusted[i].High = bar.High * ratio
		adjusted[i].Low = bar.Low * ratio
		adjusted[i].Close = bar.Close * ratio
		adjusted[i].PreClose = bar.PreClose * ratio
	}

	return adjusted, nil
}

func (t *TushareTool) getStockBasic(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	listStatus := optionalString(args, "list_status")
	if listStatus == "" {
		listStatus = "L"
	}
	stocks, err := t.client.StockBasic(ctx, listStatus)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"count":       len(stocks),
		"data_source": "tushare",
		"stocks":      stocks,
	}}, nil
}

func (t *TushareTool) getTradeCal(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	exchange := optionalString(args, "exchange")
	startDate := optionalString(args, "start_date")
	endDate := optionalString(args, "end_date")
	days, err := t.client.TradeCal(ctx, exchange, startDate, endDate)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"count":       len(days),
		"data_source": "tushare",
		"calendar":    days,
	}}, nil
}

func (t *TushareTool) getDailyBasic(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	tsCode := optionalString(args, "ts_code")
	if tsCode != "" {
		tsCode = tushare.NormalizeTsCode(tsCode)
	}
	tradeDate := optionalString(args, "trade_date")
	indicators, err := t.client.DailyBasic(ctx, tsCode, tradeDate)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"count":       len(indicators),
		"data_source": "tushare",
		"indicators":  indicators,
	}}, nil
}

func (t *TushareTool) getIncome(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	tsCode, err := requireString(args, "ts_code")
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	tsCode = tushare.NormalizeTsCode(tsCode)
	period := optionalString(args, "period")
	data, err := t.client.Income(ctx, tsCode, period)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"ts_code":     tsCode,
		"count":       len(data),
		"data_source": "tushare",
		"income":      data,
	}}, nil
}

func (t *TushareTool) getBalance(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	tsCode, err := requireString(args, "ts_code")
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	tsCode = tushare.NormalizeTsCode(tsCode)
	period := optionalString(args, "period")
	data, err := t.client.BalanceSheet(ctx, tsCode, period)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"ts_code":       tsCode,
		"count":         len(data),
		"data_source":   "tushare",
		"balance_sheet": data,
	}}, nil
}

func (t *TushareTool) getCashflow(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	tsCode, err := requireString(args, "ts_code")
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	tsCode = tushare.NormalizeTsCode(tsCode)
	period := optionalString(args, "period")
	data, err := t.client.CashFlow(ctx, tsCode, period)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"ts_code":     tsCode,
		"count":       len(data),
		"data_source": "tushare",
		"cashflow":    data,
	}}, nil
}

func (t *TushareTool) getFinaIndicator(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	tsCode, err := requireString(args, "ts_code")
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	tsCode = tushare.NormalizeTsCode(tsCode)
	period := optionalString(args, "period")
	data, err := t.client.FinaIndicator(ctx, tsCode, period)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"ts_code":     tsCode,
		"count":       len(data),
		"data_source": "tushare",
		"indicators":  data,
	}}, nil
}

func (t *TushareTool) getIndexWeight(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	indexCode, err := requireString(args, "index_code")
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	tradeDate := optionalString(args, "trade_date")
	data, err := t.client.IndexWeight(ctx, indexCode, tradeDate)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"index_code":  indexCode,
		"count":       len(data),
		"data_source": "tushare",
		"weights":     data,
	}}, nil
}

func (t *TushareTool) getDividend(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	tsCode, err := requireString(args, "ts_code")
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	tsCode = tushare.NormalizeTsCode(tsCode)
	data, err := t.client.Dividend(ctx, tsCode)
	if err != nil {
		return ToolResult{Name: "tushare", Error: err.Error()}, nil
	}
	return ToolResult{Name: "tushare", Output: map[string]interface{}{
		"ts_code":     tsCode,
		"count":       len(data),
		"data_source": "tushare",
		"dividends":   data,
	}}, nil
}
