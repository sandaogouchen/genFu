package indicator

import (
	"testing"
)

func TestCalcRSI_Basic(t *testing.T) {
	points := generateTestKlines(50, 100)
	params := DefaultRSIParams()

	result := CalcRSI(points, params)

	if len(result) != 50 {
		t.Fatalf("expected 50 RSI points, got %d", len(result))
	}

	// 前 14 个 RSI 应该为 0（预热期）
	for i := 0; i < 14; i++ {
		if result[i].Value != 0 {
			t.Errorf("point %d: expected RSI = 0 during warmup, got %f", i, result[i].Value)
		}
	}

	// 第 14 个开始应该有有效值
	if result[14].Value == 0 {
		t.Error("point 14: expected non-zero RSI")
	}
}

func TestCalcRSI_Range(t *testing.T) {
	points := generateTestKlines(100, 50)
	result := CalcRSI(points, DefaultRSIParams())

	for i := 14; i < len(result); i++ {
		if result[i].Value < 0 || result[i].Value > 100 {
			t.Errorf("point %d: RSI %f out of [0, 100] range", i, result[i].Value)
		}
	}
}

func TestCalcRSI_Zones(t *testing.T) {
	// 纯上涨数据 → RSI 应接近 100
	points := make([]KlinePoint, 30)
	price := 100.0
	for i := 0; i < 30; i++ {
		price += 2.0
		points[i] = KlinePoint{
			Timestamp: int64(1700000000 + i*86400),
			Close:     price,
			Open:      price, High: price, Low: price,
		}
	}

	result := CalcRSI(points, DefaultRSIParams())
	last := result[len(result)-1]
	if last.Zone != "overbought" {
		t.Errorf("expected overbought zone for pure uptrend, got %s (RSI=%f)", last.Zone, last.Value)
	}

	// 纯下跌数据 → RSI 应接近 0
	points2 := make([]KlinePoint, 30)
	price2 := 200.0
	for i := 0; i < 30; i++ {
		price2 -= 2.0
		points2[i] = KlinePoint{
			Timestamp: int64(1700000000 + i*86400),
			Close:     price2,
			Open:      price2, High: price2, Low: price2,
		}
	}

	result2 := CalcRSI(points2, DefaultRSIParams())
	last2 := result2[len(result2)-1]
	if last2.Zone != "oversold" {
		t.Errorf("expected oversold zone for pure downtrend, got %s (RSI=%f)", last2.Zone, last2.Value)
	}
}

func TestCalcRSI_EmptyInput(t *testing.T) {
	result := CalcRSI(nil, DefaultRSIParams())
	if len(result) != 0 {
		t.Fatalf("expected 0 results for empty input, got %d", len(result))
	}
}

func TestCalcRSI_CustomPeriod(t *testing.T) {
	points := generateTestKlines(50, 100)
	result := CalcRSI(points, RSIParams{Period: 7})

	// period=7，第 7 个开始应有有效值
	if result[7].Value == 0 {
		t.Error("point 7: expected non-zero RSI with period=7")
	}
}
