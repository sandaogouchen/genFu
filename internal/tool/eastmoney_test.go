package tool

import (
	"log"
	"strings"
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

func TestCandidateImpersonates(t *testing.T) {
	got := candidateImpersonates("chrome136")
	if len(got) == 0 {
		t.Fatalf("candidateImpersonates returned empty list")
	}
	if got[0] != "chrome136" {
		t.Fatalf("primary impersonate should be first, got=%v", got)
	}
	seen := map[string]bool{}
	for _, item := range got {
		if seen[item] {
			t.Fatalf("duplicate impersonate candidate: %s", item)
		}
		seen[item] = true
	}
	if !seen["chrome"] {
		t.Fatalf("expected fallback candidate chrome, got=%v", got)
	}
}

func TestIsUnsupportedImpersonateError(t *testing.T) {
	errText := "requests.exceptions.RequestsError: Impersonate chrome999 is not supported"
	if !isUnsupportedImpersonateError(assertErr(errText)) {
		t.Fatalf("expected unsupported impersonate to be detected")
	}
	if isUnsupportedImpersonateError(assertErr("connection closed abruptly")) {
		t.Fatalf("unexpected unsupported detection for generic transport error")
	}
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

func assertErr(msg string) error {
	return staticErr(strings.TrimSpace(msg))
}
