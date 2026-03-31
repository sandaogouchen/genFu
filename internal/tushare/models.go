package tushare

// DailyBar represents a single day/week/month OHLCV bar.
type DailyBar struct {
	TsCode    string  `json:"ts_code"`
	TradeDate string  `json:"trade_date"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	PreClose  float64 `json:"pre_close"`
	Change    float64 `json:"change"`
	PctChg    float64 `json:"pct_chg"`
	Vol       float64 `json:"vol"`
	Amount    float64 `json:"amount"`
}

// AdjFactorRow represents a daily adjustment factor.
type AdjFactorRow struct {
	TsCode    string  `json:"ts_code"`
	TradeDate string  `json:"trade_date"`
	AdjFactor float64 `json:"adj_factor"`
}

// DailyIndicator represents daily basic indicators.
type DailyIndicator struct {
	TsCode       string  `json:"ts_code"`
	TradeDate    string  `json:"trade_date"`
	Close        float64 `json:"close"`
	TurnoverRate float64 `json:"turnover_rate"`
	VolumeRatio  float64 `json:"volume_ratio"`
	PE           float64 `json:"pe"`
	PETTM        float64 `json:"pe_ttm"`
	PB           float64 `json:"pb"`
	PS           float64 `json:"ps"`
	PSTTM        float64 `json:"ps_ttm"`
	TotalShare   float64 `json:"total_share"`
	FloatShare   float64 `json:"float_share"`
	TotalMV      float64 `json:"total_mv"`
	CircMV       float64 `json:"circ_mv"`
}

// StockInfo represents basic stock information.
type StockInfo struct {
	TsCode     string `json:"ts_code"`
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	Area       string `json:"area"`
	Industry   string `json:"industry"`
	Market     string `json:"market"`
	ListDate   string `json:"list_date"`
	ListStatus string `json:"list_status"`
}

// CalendarDay represents a single trading calendar entry.
type CalendarDay struct {
	Exchange     string `json:"exchange"`
	CalDate      string `json:"cal_date"`
	IsOpen       int    `json:"is_open"`
	PreTradeDate string `json:"pretrade_date"`
}

// IndexConst represents an index constituent with weight.
type IndexConst struct {
	IndexCode string  `json:"index_code"`
	ConCode   string  `json:"con_code"`
	TradeDate string  `json:"trade_date"`
	Weight    float64 `json:"weight"`
}

// IncomeStatement represents a simplified income statement.
type IncomeStatement struct {
	TsCode       string  `json:"ts_code"`
	AnnDate      string  `json:"ann_date"`
	FAnnDate     string  `json:"f_ann_date"`
	EndDate      string  `json:"end_date"`
	ReportType   string  `json:"report_type"`
	Revenue      float64 `json:"revenue"`
	OperateProfit float64 `json:"operate_profit"`
	TotalProfit  float64 `json:"total_profit"`
	NIncome      float64 `json:"n_income"`
	NIncomeAttrP float64 `json:"n_income_attr_p"`
}

// BalanceSheetRow represents a simplified balance sheet.
type BalanceSheetRow struct {
	TsCode      string  `json:"ts_code"`
	AnnDate     string  `json:"ann_date"`
	FAnnDate    string  `json:"f_ann_date"`
	EndDate     string  `json:"end_date"`
	ReportType  string  `json:"report_type"`
	TotalAssets float64 `json:"total_assets"`
	TotalLiab   float64 `json:"total_liab"`
	TotalHldrEqyExcMinInt float64 `json:"total_hldr_eqy_exc_min_int"`
}

// CashFlowRow represents a simplified cash flow statement.
type CashFlowRow struct {
	TsCode        string  `json:"ts_code"`
	AnnDate       string  `json:"ann_date"`
	FAnnDate      string  `json:"f_ann_date"`
	EndDate       string  `json:"end_date"`
	ReportType    string  `json:"report_type"`
	NetOperateCF  float64 `json:"n_cashflow_act"`
	NetInvestCF   float64 `json:"n_cashflow_inv_act"`
	NetFinanceCF  float64 `json:"n_cash_flows_fnc_act"`
}

// FinaIndicatorRow represents key financial indicators.
type FinaIndicatorRow struct {
	TsCode     string  `json:"ts_code"`
	AnnDate    string  `json:"ann_date"`
	EndDate    string  `json:"end_date"`
	Eps        float64 `json:"eps"`
	BPS        float64 `json:"bps"`
	ROE        float64 `json:"roe"`
	ROA        float64 `json:"roa"`
	GrossMargin float64 `json:"grossprofit_margin"`
	NetMargin  float64 `json:"netprofit_margin"`
	DebtToAssets float64 `json:"debt_to_assets"`
	CurrentRatio float64 `json:"current_ratio"`
}

// DividendRow represents dividend/bonus share data.
type DividendRow struct {
	TsCode    string  `json:"ts_code"`
	EndDate   string  `json:"end_date"`
	AnnDate   string  `json:"ann_date"`
	DivProc   string  `json:"div_proc"`
	CashDiv   float64 `json:"cash_div"`
	CashDivTax float64 `json:"cash_div_tax"`
	RecordDate string  `json:"record_date"`
	ExDate     string  `json:"ex_date"`
	PayDate    string  `json:"pay_date"`
}
