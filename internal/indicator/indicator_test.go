package indicator

import (
	"testing"
)

func TestCalcAll_Default(t *testing.T) {
	points := generateTestKlines(50, 100)

	result, err := CalcAll(points)
	if err != nil {
		t.Fatalf("CalcAll failed: %v", err)
	}

	// 应该有所有三个指标
	if len(result.MACD) == 0 {
		t.Error("expected non-empty MACD")
	}
	if len(result.RSI) == 0 {
		t.Error("expected non-empty RSI")
	}
	if len(result.Bollinger) == 0 {
		t.Error("expected non-empty Bollinger")
	}

	// Latest 快照应有值
	if result.Latest.Close == 0 {
		t.Error("expected non-zero Latest.Close")
	}
	if result.Latest.Time == "" {
		t.Error("expected non-empty Latest.Time")
	}

	// Count 应为 50
	if result.Count != 50 {
		t.Errorf("expected Count=50, got %d", result.Count)
	}
}

func TestCalcAll_SelectiveMACD(t *testing.T) {
	points := generateTestKlines(50, 100)

	result, err := CalcAll(points, WithMACD(DefaultMACDParams()))
	if err != nil {
		t.Fatalf("CalcAll failed: %v", err)
	}

	if len(result.MACD) == 0 {
		t.Error("expected non-empty MACD")
	}
	if len(result.RSI) != 0 {
		t.Error("expected empty RSI when only MACD selected")
	}
	if len(result.Bollinger) != 0 {
		t.Error("expected empty Bollinger when only MACD selected")
	}
}

func TestCalcAll_EmptyInput(t *testing.T) {
	_, err := CalcAll(nil)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestCalcAll_Signals_TransitionBased(t *testing.T) {
	// 纯上涨数据：RSI 进入超买区间后，应只产生一次（transition）信号，而非每个点都产生
	points := make([]KlinePoint, 50)
	price := 100.0
	for i := 0; i < 50; i++ {
		price += 3.0
		points[i] = KlinePoint{
			Timestamp: int64(1700000000 + i*86400),
			Close:     price,
			Open:      price, High: price, Low: price,
		}
	}

	result, err := CalcAll(points)
	if err != nil {
		t.Fatalf("CalcAll failed: %v", err)
	}

	overboughtCount := 0
	for _, s := range result.Signals {
		if s.Indicator == "rsi" && s.Type == "overbought" {
			overboughtCount++
		}
	}

	if overboughtCount == 0 {
		t.Error("expected at least one RSI overbought signal in pure uptrend data")
	}

	// 信号应为少量转换事件，不应泛滥
	if overboughtCount > 3 {
		t.Errorf("expected few transition-based RSI overbought signals, got %d (possible signal flooding)", overboughtCount)
	}
}

func TestCalcAll_DataRange(t *testing.T) {
	points := []KlinePoint{
		{Timestamp: 1700000000, Close: 100, Open: 100, High: 101, Low: 99, Time: "2023-11-14"},
		{Timestamp: 1700086400, Close: 102, Open: 101, High: 103, Low: 100, Time: "2023-11-15"},
	}

	result, err := CalcAll(points)
	if err != nil {
		t.Fatalf("CalcAll failed: %v", err)
	}

	expected := "2023-11-14 ~ 2023-11-15"
	if result.DataRange != expected {
		t.Errorf("expected DataRange=%q, got %q", expected, result.DataRange)
	}
}
