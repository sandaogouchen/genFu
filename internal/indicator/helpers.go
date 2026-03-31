package indicator

import (
	"fmt"
	"math"
)

// calcEMA 计算指数移动平均线序列。
// 返回长度与 data 相同的切片。前 period-1 个元素为 0（数据不足）。
func calcEMA(data []float64, period int) []float64 {
	n := len(data)
	ema := make([]float64, n)
	if n == 0 || period <= 0 {
		return ema
	}
	if period > n {
		period = n
	}

	k := 2.0 / float64(period+1)

	// 第一个有效 EMA 使用前 period 个数据的 SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	ema[period-1] = sum / float64(period)

	// 递推
	for i := period; i < n; i++ {
		ema[i] = data[i]*k + ema[i-1]*(1-k)
	}

	return ema
}

// calcSMA 计算简单移动平均线序列。
// 返回长度与 data 相同的切片。前 period-1 个元素为 0。
func calcSMA(data []float64, period int) []float64 {
	n := len(data)
	sma := make([]float64, n)
	if n == 0 || period <= 0 {
		return sma
	}

	sum := 0.0
	for i := 0; i < n; i++ {
		sum += data[i]
		if i >= period {
			sum -= data[i-period]
		}
		if i >= period-1 {
			sma[i] = sum / float64(period)
		}
	}
	return sma
}

// calcStdDev 计算滚动标准差序列（总体标准差，除以 N）。
// 需要对应的 SMA 序列。前 period-1 个元素为 0。
func calcStdDev(data []float64, sma []float64, period int) []float64 {
	n := len(data)
	sd := make([]float64, n)
	if n == 0 || period <= 0 {
		return sd
	}

	for i := period - 1; i < n; i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := data[j] - sma[i]
			sum += diff * diff
		}
		sd[i] = math.Sqrt(sum / float64(period))
	}
	return sd
}

// timeLabel 返回 KlinePoint 的时间标签。
// 优先使用 Time 字段，否则将 Timestamp 转为字符串。
func timeLabel(p KlinePoint) string {
	if p.Time != "" {
		return p.Time
	}
	ts := p.Timestamp
	if ts > 1e12 {
		ts /= 1000
	}
	return fmt.Sprintf("%d", ts)
}

// closePrices 提取收盘价序列
func closePrices(points []KlinePoint) []float64 {
	prices := make([]float64, len(points))
	for i, p := range points {
		prices[i] = p.Close
	}
	return prices
}
