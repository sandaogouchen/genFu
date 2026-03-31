package indicator

// CalcMACD 计算 MACD 指标。
// 返回与 points 等长的 MACDPoint 切片。
// 前 slow+signal-2 个数据点为预热期，DIF/DEA/Histogram 可能为 0 或不准确，不产生信号。
func CalcMACD(points []KlinePoint, params MACDParams) []MACDPoint {
	if params.Fast <= 0 {
		params.Fast = 12
	}
	if params.Slow <= 0 {
		params.Slow = 26
	}
	if params.Signal <= 0 {
		params.Signal = 9
	}

	n := len(points)
	result := make([]MACDPoint, n)
	if n == 0 {
		return result
	}

	closes := closePrices(points)

	// EMA(fast) 和 EMA(slow)
	emaFast := calcEMA(closes, params.Fast)
	emaSlow := calcEMA(closes, params.Slow)

	// DIF = EMA(fast) - EMA(slow)
	// DIF 仅从 slow-1 开始有效（两条 EMA 都已初始化）
	dif := make([]float64, n)
	difValidFrom := params.Slow - 1 // 第一个有效 DIF 的索引
	for i := difValidFrom; i < n; i++ {
		dif[i] = emaFast[i] - emaSlow[i]
	}

	// DEA = EMA(signal) of DIF
	// 仅在有效 DIF 段上计算，避免预热期垃圾值污染 DEA
	deaFull := make([]float64, n)
	if difValidFrom < n {
		validDif := dif[difValidFrom:]
		validDea := calcEMA(validDif, params.Signal)
		copy(deaFull[difValidFrom:], validDea)
	}

	// DEA 从 difValidFrom + signal - 1 开始有效
	deaValidFrom := difValidFrom + params.Signal - 1
	// 信号检测最小有效索引：DEA 完全有效后才能判断交叉
	minSignalIdx := deaValidFrom

	// 填充结果
	for i := 0; i < n; i++ {
		result[i] = MACDPoint{
			Time:      timeLabel(points[i]),
			DIF:       dif[i],
			DEA:       deaFull[i],
			Histogram: (dif[i] - deaFull[i]) * 2,
		}

		// 信号标签：金叉/死叉
		// 仅在预热期之后检测，避免虚假信号
		if i > minSignalIdx {
			prevDifAboveDea := dif[i-1] > deaFull[i-1]
			currDifAboveDea := dif[i] > deaFull[i]
			prevDifBelowDea := dif[i-1] < deaFull[i-1]
			currDifBelowDea := dif[i] < deaFull[i]

			if !prevDifAboveDea && currDifAboveDea {
				result[i].Signal = "golden_cross"
			} else if !prevDifBelowDea && currDifBelowDea {
				result[i].Signal = "death_cross"
			}
		}
	}

	return result
}
