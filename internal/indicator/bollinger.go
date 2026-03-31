package indicator

import "sort"

// CalcBollinger 计算布林带指标。
// 返回与 points 等长的 BollingerPoint 切片。
// 前 period-1 个数据点上中下轨为 0（数据不足）。
func CalcBollinger(points []KlinePoint, params BollingerParams) []BollingerPoint {
	if params.Period <= 0 {
		params.Period = 20
	}
	if params.Multiplier <= 0 {
		params.Multiplier = 2.0
	}

	n := len(points)
	result := make([]BollingerPoint, n)
	if n == 0 {
		return result
	}

	closes := closePrices(points)
	sma := calcSMA(closes, params.Period)
	sd := calcStdDev(closes, sma, params.Period)

	// 收集所有有效带宽，用于 squeeze 判定
	bandwidths := make([]float64, 0, n)

	for i := 0; i < n; i++ {
		pt := BollingerPoint{
			Time: timeLabel(points[i]),
		}

		if i >= params.Period-1 {
			pt.Middle = sma[i]
			pt.Upper = sma[i] + params.Multiplier*sd[i]
			pt.Lower = sma[i] - params.Multiplier*sd[i]

			// %B = (Price - Lower) / (Upper - Lower)
			bandWidth := pt.Upper - pt.Lower
			if bandWidth > 0 {
				pt.PercentB = (points[i].Close - pt.Lower) / bandWidth
			}

			// 带宽 = (Upper - Lower) / Middle
			if pt.Middle > 0 {
				pt.Bandwidth = bandWidth / pt.Middle
			}

			bandwidths = append(bandwidths, pt.Bandwidth)

			// 信号标签
			if points[i].Close > pt.Upper {
				pt.Signal = "break_upper"
			} else if points[i].Close < pt.Lower {
				pt.Signal = "break_lower"
			}
		}

		result[i] = pt
	}

	// Squeeze 判定：带宽 < 近 120 根 K 线带宽的 10 分位数
	// 对最近 N 个无其他信号的有效点做 squeeze 标记
	if len(bandwidths) > 1 {
		// 取最近 120 个有效带宽
		lookback := 120
		if len(bandwidths) < lookback {
			lookback = len(bandwidths)
		}

		recentBW := make([]float64, lookback)
		copy(recentBW, bandwidths[len(bandwidths)-lookback:])
		sort.Float64s(recentBW)

		percentile10Idx := int(float64(lookback) * 0.1)
		if percentile10Idx >= lookback {
			percentile10Idx = lookback - 1
		}
		threshold := recentBW[percentile10Idx]

		// 从末尾向前扫描，标记连续处于 squeeze 状态的点（最多 5 个）
		squeezeCount := 0
		maxSqueezeMarks := 5
		for i := n - 1; i >= params.Period-1 && squeezeCount < maxSqueezeMarks; i-- {
			if result[i].Signal == "" && result[i].Bandwidth > 0 && result[i].Bandwidth < threshold {
				result[i].Signal = "squeeze"
				squeezeCount++
			} else if result[i].Signal != "squeeze" {
				// 遇到非 squeeze 点（有其他信号或不满足条件）就停止
				break
			}
		}
	}

	return result
}
