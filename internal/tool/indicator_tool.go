package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sandaogouchen/genFu/internal/indicator"
)

// IndicatorTool 技术指标计算工具，实现 Tool 接口
type IndicatorTool struct {
	registry *Registry
}

// NewIndicatorTool 创建技术指标工具
func NewIndicatorTool(registry *Registry) *IndicatorTool {
	return &IndicatorTool{registry: registry}
}

func (t *IndicatorTool) Name() string {
	return "indicator"
}

func (t *IndicatorTool) Description() string {
	return "计算股票/加密货币的技术分析指标（MACD、RSI、布林带）。输入交易对代码，返回最新指标数值和信号事件。支持自定义参数。"
}

func (t *IndicatorTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "操作类型: calculate(计算指标), list_indicators(列出可用指标), explain(解释指标)",
				"enum":        []string{"calculate", "list_indicators", "explain"},
			},
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "交易对/股票代码，如 BTCUSDT, 600519",
			},
			"period": map[string]interface{}{
				"type":        "string",
				"description": "K线周期: 1m/5m/15m/30m/1h/4h/1d/1w，默认 1d",
			},
			"indicators": map[string]interface{}{
				"type":        "string",
				"description": "要计算的指标，逗号分隔: macd,rsi,bollinger，默认全部",
			},
			"macd_fast": map[string]interface{}{
				"type":        "integer",
				"description": "MACD 快线周期，默认 12",
			},
			"macd_slow": map[string]interface{}{
				"type":        "integer",
				"description": "MACD 慢线周期，默认 26",
			},
			"macd_signal": map[string]interface{}{
				"type":        "integer",
				"description": "MACD 信号线周期，默认 9",
			},
			"rsi_period": map[string]interface{}{
				"type":        "integer",
				"description": "RSI 周期，默认 14",
			},
			"bb_period": map[string]interface{}{
				"type":        "integer",
				"description": "布林带周期，默认 20",
			},
			"bb_multiplier": map[string]interface{}{
				"type":        "number",
				"description": "布林带标准差倍数，默认 2.0",
			},
			"indicator_name": map[string]interface{}{
				"type":        "string",
				"description": "explain 操作时指定要解释的指标名称: macd/rsi/bollinger",
			},
		},
		"required": []string{"action"},
	}
}

// indicatorParams 解析后的请求参数
type indicatorParams struct {
	Action        string  `json:"action"`
	Symbol        string  `json:"symbol"`
	Period        string  `json:"period"`
	Indicators    string  `json:"indicators"`
	MACDFast      int     `json:"macd_fast"`
	MACDSlow      int     `json:"macd_slow"`
	MACDSignal    int     `json:"macd_signal"`
	RSIPeriod     int     `json:"rsi_period"`
	BBPeriod      int     `json:"bb_period"`
	BBMultiplier  float64 `json:"bb_multiplier"`
	IndicatorName string  `json:"indicator_name"`
}

func (t *IndicatorTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p indicatorParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Action {
	case "calculate":
		return t.calculate(ctx, p)
	case "list_indicators":
		return t.listIndicators()
	case "explain":
		return t.explain(p.IndicatorName)
	default:
		return "", fmt.Errorf("unknown action: %s, supported: calculate, list_indicators, explain", p.Action)
	}
}

func (t *IndicatorTool) calculate(ctx context.Context, p indicatorParams) (string, error) {
	if p.Symbol == "" {
		return "", fmt.Errorf("symbol is required for calculate action")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// 通过 Registry 获取行情数据 Tool
	mdTool, ok := t.registry.Get("get_kline_data")
	if !ok {
		return "", fmt.Errorf("market data tool (get_kline_data) not registered")
	}

	// 构造行情请求
	period := p.Period
	if period == "" {
		period = "1d"
	}

	mdParams, _ := json.Marshal(map[string]interface{}{
		"symbol": p.Symbol,
		"period": period,
		"limit":  300, // 获取 300 根 K 线以确保指标预热充分
	})

	// 获取 K 线数据
	rawKlines, err := mdTool.Execute(ctx, mdParams)
	if err != nil {
		return "", fmt.Errorf("fetch kline data failed: %w", err)
	}

	var klineData []KlineData
	if err := json.Unmarshal([]byte(rawKlines), &klineData); err != nil {
		return "", fmt.Errorf("parse kline data failed: %w", err)
	}

	if len(klineData) == 0 {
		return "", fmt.Errorf("no kline data returned for %s", p.Symbol)
	}

	// 转换为 indicator.KlinePoint
	points := make([]indicator.KlinePoint, len(klineData))
	for i, kd := range klineData {
		points[i] = indicator.KlinePoint{
			Timestamp: kd.Timestamp,
			Open:      kd.Open,
			High:      kd.High,
			Low:       kd.Low,
			Close:     kd.Close,
			Volume:    kd.Volume,
		}
	}

	// 构造 Option
	opts := t.buildOptions(p)

	// 计算指标
	result, err := indicator.CalcAll(points, opts...)
	if err != nil {
		return "", fmt.Errorf("indicator calculation failed: %w", err)
	}

	result.Symbol = p.Symbol
	result.Period = period

	// 序列化返回
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result failed: %w", err)
	}

	return string(data), nil
}

func (t *IndicatorTool) buildOptions(p indicatorParams) []indicator.Option {
	// 解析要计算的指标
	indicatorSet := map[string]bool{"macd": true, "rsi": true, "bollinger": true}
	if p.Indicators != "" {
		indicatorSet = map[string]bool{}
		for _, name := range strings.Split(p.Indicators, ",") {
			indicatorSet[strings.TrimSpace(strings.ToLower(name))] = true
		}
	}

	var opts []indicator.Option

	if indicatorSet["macd"] {
		mp := indicator.DefaultMACDParams()
		if p.MACDFast > 0 {
			mp.Fast = p.MACDFast
		}
		if p.MACDSlow > 0 {
			mp.Slow = p.MACDSlow
		}
		if p.MACDSignal > 0 {
			mp.Signal = p.MACDSignal
		}
		opts = append(opts, indicator.WithMACD(mp))
	}

	if indicatorSet["rsi"] {
		rp := indicator.DefaultRSIParams()
		if p.RSIPeriod > 0 {
			rp.Period = p.RSIPeriod
		}
		opts = append(opts, indicator.WithRSI(rp))
	}

	if indicatorSet["bollinger"] || indicatorSet["boll"] || indicatorSet["bb"] {
		bp := indicator.DefaultBollingerParams()
		if p.BBPeriod > 0 {
			bp.Period = p.BBPeriod
		}
		if p.BBMultiplier > 0 {
			bp.Multiplier = p.BBMultiplier
		}
		opts = append(opts, indicator.WithBollinger(bp))
	}

	return opts
}

func (t *IndicatorTool) listIndicators() (string, error) {
	info := map[string]interface{}{
		"indicators": []map[string]interface{}{
			{
				"name":           "MACD",
				"description":    "指数平滑异同平均线 (Moving Average Convergence Divergence)",
				"default_params": map[string]int{"fast": 12, "slow": 26, "signal": 9},
				"outputs":        []string{"DIF", "DEA", "Histogram", "Signal(golden_cross/death_cross)"},
			},
			{
				"name":           "RSI",
				"description":    "相对强弱指标 (Relative Strength Index)",
				"default_params": map[string]int{"period": 14},
				"outputs":        []string{"Value(0-100)", "Zone(overbought/oversold/neutral)"},
			},
			{
				"name":           "Bollinger Bands",
				"description":    "布林带 (Bollinger Bands)",
				"default_params": map[string]interface{}{"period": 20, "multiplier": 2.0},
				"outputs":        []string{"Upper", "Middle", "Lower", "PercentB", "Bandwidth", "Signal(break_upper/break_lower/squeeze)"},
			},
		},
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	return string(data), nil
}

func (t *IndicatorTool) explain(name string) (string, error) {
	explanations := map[string]string{
		"macd": `MACD（指数平滑异同平均线）

计算方式：
- DIF = EMA(快线) - EMA(慢线)，默认 EMA(12) - EMA(26)
- DEA = DIF 的 EMA(信号线)，默认 EMA(9)
- 柱状图 = (DIF - DEA) × 2

核心信号：
- 金叉（golden_cross）：DIF 从下方上穿 DEA，通常被视为买入信号
- 死叉（death_cross）：DIF 从上方下穿 DEA，通常被视为卖出信号
- 柱状图由负转正/由正转负也是重要的动能变化信号

使用建议：
- MACD 适合判断中期趋势方向和动能变化
- 与 RSI、布林带配合使用可提高信号可靠性`,

		"rsi": `RSI（相对强弱指标）

计算方式（Wilder 平滑法）：
- RS = 平均上涨幅度 / 平均下跌幅度
- RSI = 100 - 100/(1+RS)
- 默认周期：14

核心信号：
- 超买区（>70）：可能面临回调压力
- 超卖区（<30）：可能存在反弹机会
- 中性区（30-70）：趋势延续

使用建议：
- RSI 适合判断短期超买超卖状态
- 70/30 是经典阈值，牛市中可调至 80/40，熊市中可调至 60/20
- RSI 背离（价格新高但 RSI 未新高）是重要的反转信号`,

		"bollinger": `布林带（Bollinger Bands）

计算方式：
- 中轨 = SMA(周期)，默认 SMA(20)
- 上轨 = 中轨 + 倍数 × 标准差，默认 2 倍
- 下轨 = 中轨 - 倍数 × 标准差
- %B = (价格 - 下轨) / (上轨 - 下轨)
- 带宽 = (上轨 - 下轨) / 中轨

核心信号：
- 突破上轨（break_upper）：强势突破，可能延续或回调
- 跌破下轨（break_lower）：弱势跌破，可能延续或反弹
- 收窄（squeeze）：带宽收窄至极低水平，预示即将变盘

使用建议：
- 布林带收窄后的突破方向通常是后续趋势方向
- %B > 1 或 < 0 表示价格突破带外，需关注是否回归
- 配合成交量分析可判断突破有效性`,
	}

	key := strings.ToLower(strings.TrimSpace(name))
	if key == "boll" || key == "bb" {
		key = "bollinger"
	}

	explanation, ok := explanations[key]
	if !ok {
		return "", fmt.Errorf("unknown indicator: %s. Available: macd, rsi, bollinger", name)
	}

	return explanation, nil
}
