package analyze

import (
	"context"
	"encoding/json"

	"github.com/sandaogouchen/genFu/internal/indicator"
	"github.com/sandaogouchen/genFu/internal/tool"
)

// AnalyzeRequest 分析请求
type AnalyzeRequest struct {
	Symbol     string            `json:"symbol"`              // 交易对，例如 BTCUSDT
	Period     string            `json:"period"`              // K线周期
	Indicators []string          `json:"indicators,omitempty"` // 需要计算的技术指标
	Meta       map[string]string `json:"meta,omitempty"`      // 元数据（技术指标结果等）
}

// AnalyzeResponse 分析响应
type AnalyzeResponse struct {
	Symbol            string `json:"symbol"`
	Period            string `json:"period"`
	Analysis          string `json:"analysis"`
	TechnicalAnalysis string `json:"technical_analysis,omitempty"` // 技术指标分析结果
}

// EnrichWithIndicators 获取 K 线数据并自动计算技术指标，将结果注入 Meta。
// 这是 PRD FR4 中 enrichMarketData 增强的实现。
// 如果获取或计算失败，不阻断流程，Meta 中不会包含指标数据。
func EnrichWithIndicators(req *AnalyzeRequest, registry *tool.Registry) {
	if req.Meta == nil {
		req.Meta = make(map[string]string)
	}

	// 通过 Registry 获取行情数据
	mdTool, ok := registry.Get("get_kline_data")
	if !ok {
		return
	}

	period := req.Period
	if period == "" {
		period = "1d"
	}

	params, _ := json.Marshal(map[string]interface{}{
		"symbol": req.Symbol,
		"period": period,
		"limit":  300,
	})

	// 使用 context.Background() 而非 nil，避免潜在 panic
	rawKlines, err := mdTool.Execute(context.Background(), params)
	if err != nil {
		req.Meta["indicator_error"] = err.Error()
		return
	}

	var klineData []tool.KlineData
	if err := json.Unmarshal([]byte(rawKlines), &klineData); err != nil {
		req.Meta["indicator_error"] = "parse kline data failed: " + err.Error()
		return
	}

	if len(klineData) == 0 {
		return
	}

	// 转换并计算指标
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

	// 根据 req.Indicators 选择性计算
	opts := buildIndicatorOptions(req.Indicators)

	result, err := indicator.CalcAll(points, opts...)
	if err != nil {
		req.Meta["indicator_error"] = err.Error()
		return
	}

	result.Symbol = req.Symbol
	result.Period = period

	// 完整指标结果
	raw, _ := json.Marshal(result)
	req.Meta["indicators"] = string(raw)

	// 最新快照
	latestRaw, _ := json.Marshal(result.Latest)
	req.Meta["indicators_latest"] = string(latestRaw)

	// 信号事件
	signalsRaw, _ := json.Marshal(result.Signals)
	req.Meta["indicators_signals"] = string(signalsRaw)
}

// buildIndicatorOptions 根据用户指定的指标列表构造 Option。
// 如果列表为空或包含 "all"，返回空 opts（CalcAll 默认计算全部）。
func buildIndicatorOptions(indicators []string) []indicator.Option {
	if len(indicators) == 0 {
		return nil // CalcAll 默认计算全部
	}

	set := make(map[string]bool)
	for _, name := range indicators {
		set[name] = true
	}

	if set["all"] {
		return nil
	}

	var opts []indicator.Option
	if set["macd"] {
		opts = append(opts, indicator.WithMACD(indicator.DefaultMACDParams()))
	}
	if set["rsi"] {
		opts = append(opts, indicator.WithRSI(indicator.DefaultRSIParams()))
	}
	if set["bollinger"] || set["boll"] || set["bb"] {
		opts = append(opts, indicator.WithBollinger(indicator.DefaultBollingerParams()))
	}
	return opts
}
