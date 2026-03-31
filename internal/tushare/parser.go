package tushare

import (
	"fmt"
	"regexp"
	"strings"
)

// ----- Field lists for each API -----

var dailyBarFields = []string{
	"ts_code", "trade_date", "open", "high", "low", "close",
	"pre_close", "change", "pct_chg", "vol", "amount",
}

var adjFactorFields = []string{"ts_code", "trade_date", "adj_factor"}

var stockInfoFields = []string{
	"ts_code", "symbol", "name", "area", "industry", "market",
	"list_date", "list_status",
}

var calendarDayFields = []string{"exchange", "cal_date", "is_open", "pretrade_date"}

var dailyIndicatorFields = []string{
	"ts_code", "trade_date", "close", "turnover_rate", "volume_ratio",
	"pe", "pe_ttm", "pb", "ps", "ps_ttm",
	"total_share", "float_share", "total_mv", "circ_mv",
}

var indexConstFields = []string{"index_code", "con_code", "trade_date", "weight"}

var incomeFields = []string{
	"ts_code", "ann_date", "f_ann_date", "end_date", "report_type",
	"revenue", "operate_profit", "total_profit", "n_income", "n_income_attr_p",
}

var balanceSheetFields = []string{
	"ts_code", "ann_date", "f_ann_date", "end_date", "report_type",
	"total_assets", "total_liab", "total_hldr_eqy_exc_min_int",
}

var cashFlowFields = []string{
	"ts_code", "ann_date", "f_ann_date", "end_date", "report_type",
	"n_cashflow_act", "n_cashflow_inv_act", "n_cash_flows_fnc_act",
}

var finaIndicatorFields = []string{
	"ts_code", "ann_date", "end_date",
	"eps", "bps", "roe", "roa",
	"grossprofit_margin", "netprofit_margin",
	"debt_to_assets", "current_ratio",
}

var dividendFields = []string{
	"ts_code", "end_date", "ann_date", "div_proc",
	"cash_div", "cash_div_tax", "record_date", "ex_date", "pay_date",
}

// ----- Generic map-to-struct helpers -----

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return 0
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return 0
}

// ----- Typed parsers -----

func parseDailyBars(rows []map[string]interface{}) []DailyBar {
	out := make([]DailyBar, 0, len(rows))
	for _, r := range rows {
		out = append(out, DailyBar{
			TsCode:    getString(r, "ts_code"),
			TradeDate: getString(r, "trade_date"),
			Open:      getFloat64(r, "open"),
			High:      getFloat64(r, "high"),
			Low:       getFloat64(r, "low"),
			Close:     getFloat64(r, "close"),
			PreClose:  getFloat64(r, "pre_close"),
			Change:    getFloat64(r, "change"),
			PctChg:    getFloat64(r, "pct_chg"),
			Vol:       getFloat64(r, "vol"),
			Amount:    getFloat64(r, "amount"),
		})
	}
	return out
}

func parseAdjFactors(rows []map[string]interface{}) []AdjFactorRow {
	out := make([]AdjFactorRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, AdjFactorRow{
			TsCode:    getString(r, "ts_code"),
			TradeDate: getString(r, "trade_date"),
			AdjFactor: getFloat64(r, "adj_factor"),
		})
	}
	return out
}

func parseStockInfos(rows []map[string]interface{}) []StockInfo {
	out := make([]StockInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, StockInfo{
			TsCode:     getString(r, "ts_code"),
			Symbol:     getString(r, "symbol"),
			Name:       getString(r, "name"),
			Area:       getString(r, "area"),
			Industry:   getString(r, "industry"),
			Market:     getString(r, "market"),
			ListDate:   getString(r, "list_date"),
			ListStatus: getString(r, "list_status"),
		})
	}
	return out
}

func parseCalendarDays(rows []map[string]interface{}) []CalendarDay {
	out := make([]CalendarDay, 0, len(rows))
	for _, r := range rows {
		out = append(out, CalendarDay{
			Exchange:     getString(r, "exchange"),
			CalDate:      getString(r, "cal_date"),
			IsOpen:       getInt(r, "is_open"),
			PreTradeDate: getString(r, "pretrade_date"),
		})
	}
	return out
}

func parseDailyIndicators(rows []map[string]interface{}) []DailyIndicator {
	out := make([]DailyIndicator, 0, len(rows))
	for _, r := range rows {
		out = append(out, DailyIndicator{
			TsCode:       getString(r, "ts_code"),
			TradeDate:    getString(r, "trade_date"),
			Close:        getFloat64(r, "close"),
			TurnoverRate: getFloat64(r, "turnover_rate"),
			VolumeRatio:  getFloat64(r, "volume_ratio"),
			PE:           getFloat64(r, "pe"),
			PETTM:        getFloat64(r, "pe_ttm"),
			PB:           getFloat64(r, "pb"),
			PS:           getFloat64(r, "ps"),
			PSTTM:        getFloat64(r, "ps_ttm"),
			TotalShare:   getFloat64(r, "total_share"),
			FloatShare:   getFloat64(r, "float_share"),
			TotalMV:      getFloat64(r, "total_mv"),
			CircMV:       getFloat64(r, "circ_mv"),
		})
	}
	return out
}

func parseIndexConsts(rows []map[string]interface{}) []IndexConst {
	out := make([]IndexConst, 0, len(rows))
	for _, r := range rows {
		out = append(out, IndexConst{
			IndexCode: getString(r, "index_code"),
			ConCode:   getString(r, "con_code"),
			TradeDate: getString(r, "trade_date"),
			Weight:    getFloat64(r, "weight"),
		})
	}
	return out
}

func parseIncomeStatements(rows []map[string]interface{}) []IncomeStatement {
	out := make([]IncomeStatement, 0, len(rows))
	for _, r := range rows {
		out = append(out, IncomeStatement{
			TsCode:       getString(r, "ts_code"),
			AnnDate:      getString(r, "ann_date"),
			FAnnDate:     getString(r, "f_ann_date"),
			EndDate:      getString(r, "end_date"),
			ReportType:   getString(r, "report_type"),
			Revenue:      getFloat64(r, "revenue"),
			OperateProfit: getFloat64(r, "operate_profit"),
			TotalProfit:  getFloat64(r, "total_profit"),
			NIncome:      getFloat64(r, "n_income"),
			NIncomeAttrP: getFloat64(r, "n_income_attr_p"),
		})
	}
	return out
}

func parseBalanceSheetRows(rows []map[string]interface{}) []BalanceSheetRow {
	out := make([]BalanceSheetRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, BalanceSheetRow{
			TsCode:      getString(r, "ts_code"),
			AnnDate:     getString(r, "ann_date"),
			FAnnDate:    getString(r, "f_ann_date"),
			EndDate:     getString(r, "end_date"),
			ReportType:  getString(r, "report_type"),
			TotalAssets: getFloat64(r, "total_assets"),
			TotalLiab:   getFloat64(r, "total_liab"),
			TotalHldrEqyExcMinInt: getFloat64(r, "total_hldr_eqy_exc_min_int"),
		})
	}
	return out
}

func parseCashFlowRows(rows []map[string]interface{}) []CashFlowRow {
	out := make([]CashFlowRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, CashFlowRow{
			TsCode:       getString(r, "ts_code"),
			AnnDate:      getString(r, "ann_date"),
			FAnnDate:     getString(r, "f_ann_date"),
			EndDate:      getString(r, "end_date"),
			ReportType:   getString(r, "report_type"),
			NetOperateCF: getFloat64(r, "n_cashflow_act"),
			NetInvestCF:  getFloat64(r, "n_cashflow_inv_act"),
			NetFinanceCF: getFloat64(r, "n_cash_flows_fnc_act"),
		})
	}
	return out
}

func parseFinaIndicatorRows(rows []map[string]interface{}) []FinaIndicatorRow {
	out := make([]FinaIndicatorRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, FinaIndicatorRow{
			TsCode:      getString(r, "ts_code"),
			AnnDate:     getString(r, "ann_date"),
			EndDate:     getString(r, "end_date"),
			Eps:         getFloat64(r, "eps"),
			BPS:         getFloat64(r, "bps"),
			ROE:         getFloat64(r, "roe"),
			ROA:         getFloat64(r, "roa"),
			GrossMargin: getFloat64(r, "grossprofit_margin"),
			NetMargin:   getFloat64(r, "netprofit_margin"),
			DebtToAssets: getFloat64(r, "debt_to_assets"),
			CurrentRatio: getFloat64(r, "current_ratio"),
		})
	}
	return out
}

func parseDividendRows(rows []map[string]interface{}) []DividendRow {
	out := make([]DividendRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, DividendRow{
			TsCode:     getString(r, "ts_code"),
			EndDate:    getString(r, "end_date"),
			AnnDate:    getString(r, "ann_date"),
			DivProc:    getString(r, "div_proc"),
			CashDiv:    getFloat64(r, "cash_div"),
			CashDivTax: getFloat64(r, "cash_div_tax"),
			RecordDate: getString(r, "record_date"),
			ExDate:     getString(r, "ex_date"),
			PayDate:    getString(r, "pay_date"),
		})
	}
	return out
}

// ----- Stock code normalization -----

var (
	reSuffix = regexp.MustCompile(`^(\d{6})\.(SH|SZ|BJ)$`)
	rePrefix = regexp.MustCompile(`(?i)^(SH|SZ|BJ)(\d{6})$`)
	reDigits = regexp.MustCompile(`^\d{6}$`)
)

// NormalizeTsCode converts various stock code formats to Tushare format (e.g. 000001.SZ).
func NormalizeTsCode(code string) string {
	code = strings.TrimSpace(code)

	// Already in correct format: 000001.SZ
	if reSuffix.MatchString(strings.ToUpper(code)) {
		return strings.ToUpper(code)
	}

	// Prefix format: SZ000001 → 000001.SZ
	if m := rePrefix.FindStringSubmatch(code); m != nil {
		return m[2] + "." + strings.ToUpper(m[1])
	}

	// Plain 6 digits: infer exchange
	if reDigits.MatchString(code) {
		switch {
		case strings.HasPrefix(code, "6"):
			return code + ".SH"
		case strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3"):
			return code + ".SZ"
		case strings.HasPrefix(code, "8") || strings.HasPrefix(code, "4"):
			return code + ".BJ"
		default:
			return code + ".SZ" // fallback
		}
	}

	return code
}

// FormatDate converts "20260101" to "2026-01-01".
func FormatDate(d string) string {
	if len(d) == 8 {
		return d[:4] + "-" + d[4:6] + "-" + d[6:8]
	}
	return d
}

// UnformatDate converts "2026-01-01" to "20260101".
func UnformatDate(d string) string {
	return strings.ReplaceAll(d, "-", "")
}
