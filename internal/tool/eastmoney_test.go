package tool

import (
	"log"
	"testing"
)

func TestNormalizeSecID(t *testing.T) {
	cases := map[string]string{
		"600000":   "1.600000",
		"000001":   "0.000001",
		"300001":   "0.300001",
		"SH600000": "1.600000",
		"SZ000001": "0.000001",
		"1.600000": "1.600000",
	}
	for input, expected := range cases {
		if got := normalizeSecID(input); got != expected {
			log.Printf("normalizeSecID(%s)=%s", input, got)
			t.Fatalf("normalizeSecID(%s)=%s", input, got)
		}
	}
}

func TestScaleHelpers(t *testing.T) {
	// 东方财富API返回的价格已经是元，不需要转换
	if scalePrice(9.89) != 9.89 {
		t.Fatalf("scalePrice")
	}
	// 涨跌幅需要除以100
	if scalePercent(123) != 1.23 {
		t.Fatalf("scalePercent")
	}
	// 东方财富API返回的金额已经是元，不需要转换
	if scaleAmount(100) != 100 {
		t.Fatalf("scaleAmount")
	}
}
