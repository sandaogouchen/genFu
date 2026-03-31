package indicator

import (
	"math"
	"testing"
)

func TestCalcBollinger_Basic(t *testing.T) {
	points := generateTestKlines(50, 100)
	params := DefaultBollingerParams()

	result := CalcBollinger(points, params)

	if len(result) != 50 {
		t.Fatalf("expected 50 Bollinger points, got %d", len(result))
	}

	// 前 19 个应该全为零（period=20，需要 20 个点才有第一个有效值）
	for i := 0; i < 19; i++ {
		if result[i].Upper != 0 || result[i].Middle != 0 || result[i].Lower != 0 {
			t.Errorf("point %d: expected zero bands during warmup", i)
		}
	}

	// 第 19 个开始有效
	if result[19].Middle == 0 {
		t.Error("point 19: expected non-zero middle band")
	}
}

func TestCalcBollinger_BandOrder(t *testing.T) {
	points := generateTestKlines(50, 100)
	result := CalcBollinger(points, DefaultBollingerParams())

	for i := 19; i < len(result); i++ {
		if result[i].Upper < result[i].Middle {
			t.Errorf("point %d: upper(%f) < middle(%f)", i, result[i].Upper, result[i].Middle)
		}
		if result[i].Middle < result[i].Lower {
			t.Errorf("point %d: middle(%f) < lower(%f)", i, result[i].Middle, result[i].Lower)
		}
	}
}

func TestCalcBollinger_PercentB(t *testing.T) {
	points := generateTestKlines(50, 100)
	result := CalcBollinger(points, DefaultBollingerParams())

	for i := 19; i < len(result); i++ {
		bp := result[i]
		// %B 可以超出 [0,1]（突破上下轨时）
		// 但对于正常波动应该在合理范围内
		if math.IsNaN(bp.PercentB) || math.IsInf(bp.PercentB, 0) {
			t.Errorf("point %d: invalid PercentB: %f", i, bp.PercentB)
		}
	}
}

func TestCalcBollinger_Bandwidth(t *testing.T) {
	points := generateTestKlines(50, 100)
	result := CalcBollinger(points, DefaultBollingerParams())

	for i := 19; i < len(result); i++ {
		if result[i].Bandwidth < 0 {
			t.Errorf("point %d: negative bandwidth: %f", i, result[i].Bandwidth)
		}
	}
}

func TestCalcBollinger_BreakSignals(t *testing.T) {
	// 价格持续飙升，应该产生 break_upper 信号
	points := make([]KlinePoint, 30)
	price := 100.0
	for i := 0; i < 25; i++ {
		points[i] = KlinePoint{
			Timestamp: int64(1700000000 + i*86400),
			Close:     price + float64(i)*0.1,
			Open:      price, High: price + 1, Low: price - 1,
		}
	}
	// 最后 5 根大幅拉升
	for i := 25; i < 30; i++ {
		price += 10
		points[i] = KlinePoint{
			Timestamp: int64(1700000000 + i*86400),
			Close:     price,
			Open:      price - 5, High: price + 1, Low: price - 5,
		}
	}

	result := CalcBollinger(points, DefaultBollingerParams())

	hasBreakUpper := false
	for _, bp := range result {
		if bp.Signal == "break_upper" {
			hasBreakUpper = true
			break
		}
	}

	if !hasBreakUpper {
		t.Error("expected break_upper signal after sharp price increase")
	}
}

func TestCalcBollinger_EmptyInput(t *testing.T) {
	result := CalcBollinger(nil, DefaultBollingerParams())
	if len(result) != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", len(result))
	}
}
