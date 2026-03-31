package indicator

// KlinePoint 是指标计算的输入数据点，与 tool.KlineData 对齐。
// 调用方可自行将 tool.KlineData 转换为 KlinePoint，
// 或在 indicator 包外做适配（见 Convert 函数）。
type KlinePoint struct {
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	Time      string  `json:"time,omitempty"` // 可选的人类可读时间，如 "2026-03-31"
}

// ---------- MACD ----------

// MACDParams MACD 参数
type MACDParams struct {
	Fast   int // 快线周期，默认 12
	Slow   int // 慢线周期，默认 26
	Signal int // 信号线周期，默认 9
}

// DefaultMACDParams 返回默认 MACD 参数
func DefaultMACDParams() MACDParams {
	return MACDParams{Fast: 12, Slow: 26, Signal: 9}
}

// MACDPoint 单根 K 线的 MACD 计算结果
type MACDPoint struct {
	Time      string  `json:"time"`
	DIF       float64 `json:"dif"`
	DEA       float64 `json:"dea"`
	Histogram float64 `json:"histogram"`
	Signal    string  `json:"signal,omitempty"` // "golden_cross" / "death_cross" / ""
}

// ---------- RSI ----------

// RSIParams RSI 参数
type RSIParams struct {
	Period int // 周期，默认 14
}

// DefaultRSIParams 返回默认 RSI 参数
func DefaultRSIParams() RSIParams {
	return RSIParams{Period: 14}
}

// RSIPoint 单根 K 线的 RSI 计算结果
type RSIPoint struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"` // 0-100
	Zone  string  `json:"zone"`  // "overbought"(>70) / "oversold"(<30) / "neutral"
}

// ---------- Bollinger Bands ----------

// BollingerParams 布林带参数
type BollingerParams struct {
	Period     int     // 周期，默认 20
	Multiplier float64 // 标准差倍数，默认 2.0
}

// DefaultBollingerParams 返回默认布林带参数
func DefaultBollingerParams() BollingerParams {
	return BollingerParams{Period: 20, Multiplier: 2.0}
}

// BollingerPoint 单根 K 线的布林带计算结果
type BollingerPoint struct {
	Time      string  `json:"time"`
	Upper     float64 `json:"upper"`
	Middle    float64 `json:"middle"`
	Lower     float64 `json:"lower"`
	PercentB  float64 `json:"percent_b"`          // 0~1 为正常，<0 跌破下轨，>1 突破上轨
	Bandwidth float64 `json:"bandwidth"`           // 带宽百分比
	Signal    string  `json:"signal,omitempty"`    // "break_upper"/"break_lower"/"squeeze"/""
}

// ---------- 统一输出 ----------

// IndicatorResult 所有指标的聚合输出
type IndicatorResult struct {
	Symbol    string           `json:"symbol"`
	Period    string           `json:"period"`     // "daily"/"weekly"/...
	DataRange string           `json:"data_range"` // "2025-01-01 ~ 2026-03-31"
	Count     int              `json:"count"`
	MACD      []MACDPoint      `json:"macd,omitempty"`
	RSI       []RSIPoint       `json:"rsi,omitempty"`
	Bollinger []BollingerPoint `json:"bollinger,omitempty"`
	Signals   []SignalEvent    `json:"signals"`
	Latest    LatestSnapshot   `json:"latest"`
}

// SignalEvent 信号事件
type SignalEvent struct {
	Time      string `json:"time"`
	Indicator string `json:"indicator"` // "macd"/"rsi"/"bollinger"
	Type      string `json:"type"`      // "golden_cross"/"overbought"/"squeeze"/...
	Detail    string `json:"detail"`    // 人类可读描述
}

// LatestSnapshot 最新一根 K 线的指标快照
type LatestSnapshot struct {
	Time        string  `json:"time"`
	Close       float64 `json:"close"`
	MACD_DIF    float64 `json:"macd_dif"`
	MACD_DEA    float64 `json:"macd_dea"`
	MACD_Hist   float64 `json:"macd_histogram"`
	RSI         float64 `json:"rsi"`
	RSI_Zone    string  `json:"rsi_zone"`
	BB_Upper    float64 `json:"bb_upper"`
	BB_Middle   float64 `json:"bb_middle"`
	BB_Lower    float64 `json:"bb_lower"`
	BB_PercentB float64 `json:"bb_percent_b"`
}
