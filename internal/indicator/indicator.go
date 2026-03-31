package indicator

import (
	"fmt"
)

// Indicator 可扩展指标接口（为后续 KDJ/ATR/VWAP 等预留）
type Indicator interface {
	Name() string
	Calc(points []KlinePoint) (interface{}, error)
}

// ---------- Option 模式 ----------

type calcOptions struct {
	macd       bool
	macdP      MACDParams
	rsi        bool
	rsiP       RSIParams
	bollinger  bool
	bollingerP BollingerParams
}

// Option 配置选项
type Option func(*calcOptions)

// WithMACD 启用 MACD 计算
func WithMACD(params MACDParams) Option {
	return func(o *calcOptions) {
		o.macd = true
		o.macdP = params
	}
}

// WithRSI 启用 RSI 计算
func WithRSI(params RSIParams) Option {
	return func(o *calcOptions) {
		o.rsi = true
		o.rsiP = params
	}
}

// WithBollinger 启用布林带计算
func WithBollinger(params BollingerParams) Option {
	return func(o *calcOptions) {
		o.bollinger = true
		o.bollingerP = params
	}
}

// CalcAll 一键计算所有指标（或通过 Option 选择性计算）。
// 不传 Option 时默认计算全部三个指标。
// 注意：返回的 IndicatorResult 中 Symbol 和 Period 需要调用方设置。
func CalcAll(points []KlinePoint, opts ...Option) (*IndicatorResult, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("empty kline data")
	}

	o := &calcOptions{}

	if len(opts) > 0 {
		for _, fn := range opts {
			fn(o)
		}
	} else {
		// 默认全部计算
		o.macd = true
		o.macdP = DefaultMACDParams()
		o.rsi = true
		o.rsiP = DefaultRSIParams()
		o.bollinger = true
		o.bollingerP = DefaultBollingerParams()
	}

	result := &IndicatorResult{
		Count:   len(points),
		Signals: make([]SignalEvent, 0),
	}

	// 数据范围
	first := timeLabel(points[0])
	last := timeLabel(points[len(points)-1])
	result.DataRange = first + " ~ " + last

	// 最新快照基础
	lastPt := points[len(points)-1]
	result.Latest = LatestSnapshot{
		Time:  timeLabel(lastPt),
		Close: lastPt.Close,
	}

	// MACD
	if o.macd {
		macdPoints := CalcMACD(points, o.macdP)
		result.MACD = macdPoints

		// 收集信号（仅交叉事件，已是事件型信号）
		for _, mp := range macdPoints {
			if mp.Signal != "" {
				result.Signals = append(result.Signals, SignalEvent{
					Time:      mp.Time,
					Indicator: "macd",
					Type:      mp.Signal,
					Detail:    macdSignalDetail(mp),
				})
			}
		}

		// 更新最新快照
		if len(macdPoints) > 0 {
			lp := macdPoints[len(macdPoints)-1]
			result.Latest.MACD_DIF = lp.DIF
			result.Latest.MACD_DEA = lp.DEA
			result.Latest.MACD_Hist = lp.Histogram
		}
	}

	// RSI
	if o.rsi {
		rsiPoints := CalcRSI(points, o.rsiP)
		result.RSI = rsiPoints

		// 收集信号：仅在区间 *发生变化* 时产生事件（状态转换型），避免信号泛滥
		for i, rp := range rsiPoints {
			if rp.Zone == "overbought" || rp.Zone == "oversold" {
				// 第一个有效点：直接发出信号
				if i == 0 {
					result.Signals = append(result.Signals, SignalEvent{
						Time:      rp.Time,
						Indicator: "rsi",
						Type:      rp.Zone,
						Detail:    rsiSignalDetail(rp),
					})
				} else if rsiPoints[i-1].Zone != rp.Zone {
					// 状态发生变化时才产生信号
					result.Signals = append(result.Signals, SignalEvent{
						Time:      rp.Time,
						Indicator: "rsi",
						Type:      rp.Zone,
						Detail:    rsiSignalDetail(rp),
					})
				}
			}
		}

		// 更新最新快照
		if len(rsiPoints) > 0 {
			lp := rsiPoints[len(rsiPoints)-1]
			result.Latest.RSI = lp.Value
			result.Latest.RSI_Zone = lp.Zone
		}
	}

	// Bollinger
	if o.bollinger {
		bbPoints := CalcBollinger(points, o.bollingerP)
		result.Bollinger = bbPoints

		// 收集信号：仅在信号 *发生变化* 时产生事件，避免连续突破期间信号泛滥
		for i, bp := range bbPoints {
			if bp.Signal != "" {
				if i == 0 {
					result.Signals = append(result.Signals, SignalEvent{
						Time:      bp.Time,
						Indicator: "bollinger",
						Type:      bp.Signal,
						Detail:    bollingerSignalDetail(bp),
					})
				} else if bbPoints[i-1].Signal != bp.Signal {
					result.Signals = append(result.Signals, SignalEvent{
						Time:      bp.Time,
						Indicator: "bollinger",
						Type:      bp.Signal,
						Detail:    bollingerSignalDetail(bp),
					})
				}
			}
		}

		// 更新最新快照
		if len(bbPoints) > 0 {
			lp := bbPoints[len(bbPoints)-1]
			result.Latest.BB_Upper = lp.Upper
			result.Latest.BB_Middle = lp.Middle
			result.Latest.BB_Lower = lp.Lower
			result.Latest.BB_PercentB = lp.PercentB
		}
	}

	return result, nil
}

// ---------- 信号描述辅助 ----------

func macdSignalDetail(mp MACDPoint) string {
	switch mp.Signal {
	case "golden_cross":
		return fmt.Sprintf("MACD 金叉：DIF(%.4f) 上穿 DEA(%.4f)", mp.DIF, mp.DEA)
	case "death_cross":
		return fmt.Sprintf("MACD 死叉：DIF(%.4f) 下穿 DEA(%.4f)", mp.DIF, mp.DEA)
	default:
		return ""
	}
}

func rsiSignalDetail(rp RSIPoint) string {
	switch rp.Zone {
	case "overbought":
		return fmt.Sprintf("RSI 超买：RSI = %.2f (>70)", rp.Value)
	case "oversold":
		return fmt.Sprintf("RSI 超卖：RSI = %.2f (<30)", rp.Value)
	default:
		return ""
	}
}

func bollingerSignalDetail(bp BollingerPoint) string {
	switch bp.Signal {
	case "break_upper":
		return fmt.Sprintf("布林带突破上轨：收盘价突破上轨 %.4f", bp.Upper)
	case "break_lower":
		return fmt.Sprintf("布林带跌破下轨：收盘价跌破下轨 %.4f", bp.Lower)
	case "squeeze":
		return fmt.Sprintf("布林带收窄：带宽 %.4f 低于 10%%分位数，预示变盘", bp.Bandwidth)
	default:
		return ""
	}
}
