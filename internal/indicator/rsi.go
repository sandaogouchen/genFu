package indicator

// CalcRSI 使用 Wilder 平滑法计算 RSI 指标。
// 返回与 points 等长的 RSIPoint 切片。
// 前 period 个数据点 Value 为 0（需要 period+1 个价格才能计算第一个 RSI）。
func CalcRSI(points []KlinePoint, params RSIParams) []RSIPoint {
	if params.Period <= 0 {
		params.Period = 14
	}

	n := len(points)
	result := make([]RSIPoint, n)
	if n < 2 {
		for i := range result {
			result[i] = RSIPoint{Time: timeLabel(points[i]), Zone: "neutral"}
		}
		return result
	}

	// 计算价格变动
	changes := make([]float64, n)
	for i := 1; i < n; i++ {
		changes[i] = points[i].Close - points[i-1].Close
	}

	period := params.Period
	if period >= n {
		// 数据不足，返回空结果
		for i := range result {
			result[i] = RSIPoint{Time: timeLabel(points[i]), Zone: "neutral"}
		}
		return result
	}

	// 第一个 RSI：使用前 period 个变动的简单平均
	var avgGain, avgLoss float64
	for i := 1; i <= period; i++ {
		if changes[i] > 0 {
			avgGain += changes[i]
		} else {
			avgLoss += -changes[i]
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// 填充前 period 个为空
	for i := 0; i < period; i++ {
		result[i] = RSIPoint{Time: timeLabel(points[i]), Zone: "neutral"}
	}

	// 第 period 个位置的 RSI
	rsi := calcRSIValue(avgGain, avgLoss)
	result[period] = RSIPoint{
		Time:  timeLabel(points[period]),
		Value: rsi,
		Zone:  rsiZone(rsi),
	}

	// Wilder 平滑法递推
	for i := period + 1; i < n; i++ {
		gain := 0.0
		loss := 0.0
		if changes[i] > 0 {
			gain = changes[i]
		} else {
			loss = -changes[i]
		}

		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)

		rsi = calcRSIValue(avgGain, avgLoss)
		result[i] = RSIPoint{
			Time:  timeLabel(points[i]),
			Value: rsi,
			Zone:  rsiZone(rsi),
		}
	}

	return result
}

func calcRSIValue(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50 // 无变动
		}
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

func rsiZone(rsi float64) string {
	if rsi > 70 {
		return "overbought"
	}
	if rsi < 30 {
		return "oversold"
	}
	return "neutral"
}
