package indicator

import (
	"math"
	"testing"
)

// 生成测试用 K 线数据（模拟上升趋势）
func generateTestKlines(n int, startPrice float64) []KlinePoint {
	points := make([]KlinePoint, n)
	price := startPrice
	for i := 0; i < n; i++ {
		// 带一定波动的上升趋势
		change := math.Sin(float64(i)*0.3)*2 + 0.5
		price += change
		points[i] = KlinePoint{
			Timestamp: int64(1700000000 + i*86400),
			Time:      "",
			Open:      price - 0.5,
			High:      price + 1.0,
			Low:       price - 1.0,
			Close:     price,
			Volume:    1000 + float64(i)*10,
		}
	}
	return points
}

func TestCalcMACD_Basic(t *testing.T) {
	points := generateTestKlines(50, 100)
	params := DefaultMACDParams()

	result := CalcMACD(points, params)

	if len(result) != 50 {
		t.Fatalf("expected 50 MACD points, got %d", len(result))
	}

	// 验证最后一个点有有效值
	last := result[len(result)-1]
	if last.DIF == 0 && last.DEA == 0 {
		t.Error("last MACD point should have non-zero DIF/DEA for trending data")
	}

	// 上升趋势下，DIF 应该 > 0
	if last.DIF <= 0 {
		t.Errorf("expected positive DIF in uptrend, got %f", last.DIF)
	}
}

func TestCalcMACD_GoldenCross(t *testing.T) {
	// 构造先下后上的数据，应该产生金叉
	points := make([]KlinePoint, 80)
	price := 100.0
	for i := 0; i < 40; i++ {
		price -= 1.0
		points[i] = KlinePoint{
			Timestamp: int64(1700000000 + i*86400),
			Close:     price,
			Open:      price, High: price + 0.5, Low: price - 0.5,
		}
	}
	for i := 40; i < 80; i++ {
		price += 1.5
		points[i] = KlinePoint{
			Timestamp: int64(1700000000 + i*86400),
			Close:     price,
			Open:      price, High: price + 0.5, Low: price - 0.5,
		}
	}

	result := CalcMACD(points, DefaultMACDParams())

	hasGoldenCross := false
	// 金叉应该只在预热期之后出现
	minValid := 26 + 9 - 2 // slow + signal - 2 = 33
	for i, mp := range result {
		if mp.Signal == "golden_cross" {
			hasGoldenCross = true
			if i <= minValid {
				t.Errorf("got golden_cross at index %d which is in warmup zone (min valid: %d)", i, minValid+1)
			}
		}
	}

	if !hasGoldenCross {
		t.Error("expected golden_cross signal in V-shaped recovery data")
	}
}

func TestCalcMACD_NoSpuriousSignalsInWarmup(t *testing.T) {
	// 验证预热期内不产生虚假信号
	points := generateTestKlines(50, 100)
	result := CalcMACD(points, DefaultMACDParams())

	minValid := 26 + 9 - 2 // slow + signal - 2 = 33
	for i := 0; i <= minValid; i++ {
		if result[i].Signal != "" {
			t.Errorf("point %d: expected no signal in warmup zone, got %q", i, result[i].Signal)
		}
	}
}

func TestCalcMACD_EmptyInput(t *testing.T) {
	result := CalcMACD(nil, DefaultMACDParams())
	if len(result) != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", len(result))
	}
}

func TestCalcMACD_Histogram(t *testing.T) {
	points := generateTestKlines(50, 100)
	result := CalcMACD(points, DefaultMACDParams())

	for i, mp := range result {
		expected := (mp.DIF - mp.DEA) * 2
		if math.Abs(mp.Histogram-expected) > 1e-10 {
			t.Errorf("point %d: histogram mismatch: got %f, expected %f", i, mp.Histogram, expected)
		}
	}
}

func TestCalcMACD_DEANotSeededOnGarbage(t *testing.T) {
	// 验证 DEA 在 slow-1 之前全部为 0（不被垃圾 DIF 污染）
	points := generateTestKlines(50, 100)
	result := CalcMACD(points, DefaultMACDParams())

	slow := 26
	for i := 0; i < slow-1; i++ {
		if result[i].DEA != 0 {
			t.Errorf("point %d: expected DEA=0 before slow period, got %f", i, result[i].DEA)
		}
	}
}
