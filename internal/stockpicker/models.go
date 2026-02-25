package stockpicker

import "time"

// StockPickRequest 选股请求
type StockPickRequest struct {
	AccountID int64     `json:"account_id"`
	TopN      int       `json:"top_n"`     // 返回股票数量，默认5
	DateFrom  time.Time `json:"date_from"` // 数据起始日期
	DateTo    time.Time `json:"date_to"`   // 数据结束日期
}

// StockPickResponse 选股响应
type StockPickResponse struct {
	PickID        string           `json:"pick_id"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Stocks        []StockPick      `json:"stocks"`
	MarketData    MarketData       `json:"market_data"`
	NewsSummary   string           `json:"news_summary"`
	Warnings      []string         `json:"warnings,omitempty"`
	ScreeningInfo *ScreeningResult `json:"screening_info,omitempty"` // 筛选信息
	StrategyGuide *StrategyGuide   `json:"strategy_guide,omitempty"` // 策略级买卖行动指南
}

// StockPick 单只选股结果
type StockPick struct {
	Symbol            string             `json:"symbol"`
	Name              string             `json:"name"`
	Industry          string             `json:"industry"`
	CurrentPrice      float64            `json:"current_price"`
	Recommendation    string             `json:"recommendation"` // buy/watch
	Confidence        float64            `json:"confidence"`
	TechnicalReasons  TechnicalReason    `json:"technical_reasons"`
	FinancialAnalysis *FinancialAnalysis `json:"financial_analysis,omitempty"`
	OperationGuide    *OperationGuide    `json:"operation_guide,omitempty"`
	RiskLevel         string             `json:"risk_level"` // low/medium/high
	Allocation        Allocation         `json:"allocation"`
}

// TechnicalReason 技术面原因
type TechnicalReason struct {
	Trend               string   `json:"trend"`                // 趋势描述
	VolumeSignal        string   `json:"volume_signal"`        // 成交量信号
	TechnicalIndicators []string `json:"technical_indicators"` // 技术指标分析
	KeyLevels           []string `json:"key_levels"`           // 关键价位
	RiskPoints          []string `json:"risk_points"`          // 风险点
}

// FinancialAnalysis 财报分析
type FinancialAnalysis struct {
	ReportTitle   string   `json:"report_title"`
	ReportType    string   `json:"report_type"`
	Period        string   `json:"period"`
	Summary       string   `json:"summary"`
	KeyMetrics    Metrics  `json:"key_metrics"`
	RiskFactors   []string `json:"risk_factors"`
	GrowthDrivers []string `json:"growth_drivers"`
}

// Metrics 关键财务指标
type Metrics struct {
	Revenue       string `json:"revenue"`        // 营收
	RevenueGrowth string `json:"revenue_growth"` // 营收增长
	NetProfit     string `json:"net_profit"`     // 净利润
	ProfitGrowth  string `json:"profit_growth"`  // 利润增长
	GrossMargin   string `json:"gross_margin"`   // 毛利率
	NetMargin     string `json:"net_margin"`     // 净利率
	ROE           string `json:"roe"`            // 净资产收益率
	EPS           string `json:"eps"`            // 每股收益
}

// Allocation 资产配置建议
type Allocation struct {
	SuggestedWeight        float64 `json:"suggested_weight"`         // 建议权重
	IndustryDiversity      float64 `json:"industry_diversity"`       // 行业分散度
	RiskExposure           float64 `json:"risk_exposure"`            // 风险敞口
	LiquidityScore         float64 `json:"liquidity_score"`          // 流动性评分
	CorrelationWithHolding float64 `json:"correlation_with_holding"` // 与持仓相关性
}

// MarketData 大盘数据摘要
type MarketData struct {
	IndexQuotes     []IndexQuote `json:"index_quotes"`
	MarketSentiment string       `json:"market_sentiment"` // 市场情绪
	UpCount         int          `json:"up_count"`         // 上涨家数
	DownCount       int          `json:"down_count"`       // 下跌家数
	LimitUp         int          `json:"limit_up"`         // 涨停数
	LimitDown       int          `json:"limit_down"`       // 跌停数
}

// IndexQuote 指数行情
type IndexQuote struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	ChangeRate float64 `json:"change_rate"`
	Amount     float64 `json:"amount,omitempty"`
}

// NewsEvent 简化的新闻事件
type NewsEvent struct {
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Domains     []string  `json:"domains"`
	Direction   string    `json:"direction"` // bullish/bearish/mixed
	Priority    int       `json:"priority"`  // 1-5
	PublishedAt time.Time `json:"published_at"`
}

// Position 简化的持仓信息
type Position struct {
	Symbol   string  `json:"symbol"`
	Name     string  `json:"name"`
	Industry string  `json:"industry"`
	Quantity float64 `json:"quantity"`
	Value    float64 `json:"value"`
}

// AgentOutput Agent输出结构
type AgentOutput struct {
	Stocks        []StockPick    `json:"stocks"`
	MarketView    string         `json:"market_view"`
	RiskNotes     string         `json:"risk_notes"`
	StrategyGuide *StrategyGuide `json:"strategy_guide,omitempty"`
}

// StrategyGuide 策略级行动指南
type StrategyGuide struct {
	StrategyType     string `json:"strategy_type,omitempty"`
	StrategyName     string `json:"strategy_name,omitempty"`
	GuideText        string `json:"guide_text"`         // 行动指南文字描述
	TradeSignalsJSON string `json:"trade_signals_json"` // 严格 JSON 格式的买卖信号字符串
}

// OperationGuide 操作指南
type OperationGuide struct {
	ID             int64       `json:"id,omitempty"`
	Symbol         string      `json:"symbol"`
	PickID         string      `json:"pick_id,omitempty"`
	BuyConditions  []Condition `json:"buy_conditions"`
	SellConditions []Condition `json:"sell_conditions"`
	StopLoss       string      `json:"stop_loss"`
	TakeProfit     string      `json:"take_profit"`
	RiskMonitors   []string    `json:"risk_monitors"`
	ValidUntil     *time.Time  `json:"valid_until,omitempty"`
	CreatedAt      time.Time   `json:"created_at,omitempty"`
	UpdatedAt      time.Time   `json:"updated_at,omitempty"`
}

// Condition 操作条件
type Condition struct {
	Type        string `json:"type"`            // price/news/technical/fundamental
	Description string `json:"description"`     // 条件描述
	Value       string `json:"value,omitempty"` // 具体值
}
