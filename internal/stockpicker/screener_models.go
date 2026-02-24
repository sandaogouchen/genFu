package stockpicker

import "time"

// ScreeningRequest 筛选请求
type ScreeningRequest struct {
	StrategyType string `json:"strategy_type,omitempty"`

	// 价格条件
	PriceMin      *float64 `json:"price_min,omitempty"`
	PriceMax      *float64 `json:"price_max,omitempty"`
	ChangeRateMin *float64 `json:"change_rate_min,omitempty"`
	ChangeRateMax *float64 `json:"change_rate_max,omitempty"`

	// 成交量条件
	AmountMin *float64 `json:"amount_min,omitempty"`
	AmountMax *float64 `json:"amount_max,omitempty"`

	// 技术指标条件
	MA5AboveMA20  *bool `json:"ma5_above_ma20,omitempty"`   // 5日均线上穿20日均线
	MA20Rising    *bool `json:"ma20_rising,omitempty"`      // 20日均线向上
	MACDGolden    *bool `json:"macd_golden,omitempty"`      // MACD金叉
	RSIOversold   *bool `json:"rsi_oversold,omitempty"`     // RSI超卖(<30)
	RSIOverbought *bool `json:"rsi_overbought,omitempty"`   // RSI超买(>70)
	VolumeSpike   *bool `json:"volume_spike,omitempty"`     // 放量(成交量>5日均量2倍)

	// 筛选结果控制
	Limit int `json:"limit,omitempty"`
}

// ScreeningResult 筛选结果
type ScreeningResult struct {
	StrategyType   string          `json:"strategy_type"`
	TotalMatched   int             `json:"total_matched"`
	ReturnedCount  int             `json:"returned_count"`
	Stocks         []ScreenedStock `json:"stocks"`
	AppliedFilters []string        `json:"applied_filters"`
	ScreenedAt     time.Time       `json:"screened_at"`
}

// ScreenedStock 筛选后的股票
type ScreenedStock struct {
	Symbol        string          `json:"symbol"`
	Name          string          `json:"name"`
	Price         float64         `json:"price"`
	ChangeRate    float64         `json:"change_rate"`
	Amount        float64         `json:"amount"`
	Amplitude     float64         `json:"amplitude"`
	MatchScore    float64         `json:"match_score"`
	MatchReasons  []string        `json:"match_reasons"`
	TechnicalInfo *TechnicalInfo  `json:"technical_info,omitempty"`
}

// TechnicalInfo 技术指标信息
type TechnicalInfo struct {
	MA5         float64 `json:"ma5"`
	MA10        float64 `json:"ma10"`
	MA20        float64 `json:"ma20"`
	MACD        float64 `json:"macd"`
	MACDSignal  float64 `json:"macd_signal"`
	RSI         float64 `json:"rsi"`
	VolumeRatio float64 `json:"volume_ratio"` // 量比
}

// AgentScreeningOutput 筛选Agent输出
type AgentScreeningOutput struct {
	StrategyName        string           `json:"strategy_name"`
	StrategyDescription string           `json:"strategy_description"`
	ScreeningConditions ScreeningRequest `json:"screening_conditions"`
	MarketContext       string           `json:"market_context"`
	RiskNotes           string           `json:"risk_notes"`
}
