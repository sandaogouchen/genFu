package stockpicker

import (
	"math"
)

// CalculateMA 计算移动平均线
// prices: 价格序列，从旧到新
// period: 周期
func CalculateMA(prices []float64, period int) float64 {
	if len(prices) < period || period <= 0 {
		return 0
	}
	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += prices[i]
	}
	return sum / float64(period)
}

// CalculateEMA 计算指数移动平均线
func CalculateEMA(prices []float64, period int) float64 {
	if len(prices) < period || period <= 0 {
		return 0
	}
	multiplier := 2.0 / float64(period+1)
	ema := CalculateMA(prices[:period], period)
	for i := period; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}
	return ema
}

// MACDResult MACD计算结果
type MACDResult struct {
	MACD      float64
	Signal    float64
	Histogram float64
}

// CalculateMACD 计算MACD指标
// 标准参数: 快线12, 慢线26, 信号线9
func CalculateMACD(prices []float64) MACDResult {
	if len(prices) < 26 {
		return MACDResult{}
	}

	// 计算快线EMA12和慢线EMA26
	ema12 := CalculateEMA(prices, 12)
	ema26 := CalculateEMA(prices, 26)

	// DIF线 (MACD线)
	macd := ema12 - ema26

	// 计算DEA线 (信号线) - 需要历史DIF值
	// 简化处理：使用当前DIF作为信号线的近似
	// 更精确的计算需要保存历史DIF序列
	difSeries := make([]float64, 0)
	for i := 0; i < len(prices); i++ {
		if i >= 25 { // 需要至少26个数据点
			ema12i := CalculateEMA(prices[:i+1], 12)
			ema26i := CalculateEMA(prices[:i+1], 26)
			difSeries = append(difSeries, ema12i-ema26i)
		}
	}

	var signal float64
	if len(difSeries) >= 9 {
		signal = CalculateEMA(difSeries, 9)
	}

	return MACDResult{
		MACD:      macd,
		Signal:    signal,
		Histogram: macd - signal,
	}
}

// CalculateRSI 计算RSI指标
// period: 周期，通常为14
func CalculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 || period <= 0 {
		return 50 // 默认中性值
	}

	gains := make([]float64, 0)
	losses := make([]float64, 0)

	// 计算价格变化
	for i := 1; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains = append(gains, change)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -change)
		}
	}

	if len(gains) < period {
		return 50
	}

	// 计算平均涨跌幅
	avgGain := 0.0
	avgLoss := 0.0
	for i := len(gains) - period; i < len(gains); i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// 计算RSI
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// CalculateVolumeRatio 计算量比
// 当前成交量 / 5日平均成交量
func CalculateVolumeRatio(volumes []float64) float64 {
	if len(volumes) < 6 {
		return 1.0 // 默认值
	}

	// 当前成交量（最后一个）
	currentVolume := volumes[len(volumes)-1]

	// 5日平均成交量
	avgVolume := 0.0
	for i := len(volumes) - 6; i < len(volumes)-1; i++ {
		avgVolume += volumes[i]
	}
	avgVolume /= 5

	if avgVolume == 0 {
		return 1.0
	}

	return currentVolume / avgVolume
}

// IsMACDGoldenCross 判断MACD金叉
// 当前MACD > 信号线，且前一日MACD <= 信号线
func IsMACDGoldenCross(macdHistory []MACDResult) bool {
	if len(macdHistory) < 2 {
		return false
	}
	current := macdHistory[len(macdHistory)-1]
	previous := macdHistory[len(macdHistory)-2]

	return current.MACD > current.Signal && previous.MACD <= previous.Signal
}

// IsMARising 判断均线是否向上
// 比较当前MA与前一日MA
func IsMARising(maHistory []float64) bool {
	if len(maHistory) < 2 {
		return false
	}
	return maHistory[len(maHistory)-1] > maHistory[len(maHistory)-2]
}

// IsMA5AboveMA20 判断MA5是否在MA20上方
func IsMA5AboveMA20(ma5, ma20 float64) bool {
	return ma5 > ma20
}

// IsVolumeSpike 判断是否放量
// 量比 > 2
func IsVolumeSpike(volumeRatio float64) bool {
	return volumeRatio > 2.0
}

// IsRSIOversold 判断RSI是否超卖
func IsRSIOversold(rsi float64) bool {
	return rsi < 30
}

// IsRSIOverbought 判断RSI是否超买
func IsRSIOverbought(rsi float64) bool {
	return rsi > 70
}

// CalculateAllIndicators 计算所有技术指标
func CalculateAllIndicators(prices, volumes []float64) *TechnicalInfo {
	if len(prices) < 30 || len(volumes) < 6 {
		return nil
	}

	ma5 := CalculateMA(prices, 5)
	ma10 := CalculateMA(prices, 10)
	ma20 := CalculateMA(prices, 20)
	macdResult := CalculateMACD(prices)
	rsi := CalculateRSI(prices, 14)
	volumeRatio := CalculateVolumeRatio(volumes)

	return &TechnicalInfo{
		MA5:         roundTo2Decimal(ma5),
		MA10:        roundTo2Decimal(ma10),
		MA20:        roundTo2Decimal(ma20),
		MACD:        roundTo4Decimal(macdResult.MACD),
		MACDSignal:  roundTo4Decimal(macdResult.Signal),
		RSI:         roundTo2Decimal(rsi),
		VolumeRatio: roundTo2Decimal(volumeRatio),
	}
}

func roundTo2Decimal(v float64) float64 {
	return math.Round(v*100) / 100
}

func roundTo4Decimal(v float64) float64 {
	return math.Round(v*10000) / 10000
}
