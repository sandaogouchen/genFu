package tushare

import "testing"

func TestNormalizeTsCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"000001", "000001.SZ"},
		{"600000", "600000.SH"},
		{"300001", "300001.SZ"},
		{"830001", "830001.BJ"},
		{"430001", "430001.BJ"},
		{"000001.SZ", "000001.SZ"},
		{"600000.SH", "600000.SH"},
		{"SZ000001", "000001.SZ"},
		{"sz000001", "000001.SZ"},
		{"SH600000", "600000.SH"},
		{"sh600000", "600000.SH"},
		{"BJ830001", "830001.BJ"},
		{"  000001.SZ  ", "000001.SZ"},
	}

	for _, tt := range tests {
		got := NormalizeTsCode(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeTsCode(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"20260101", "2026-01-01"},
		{"20251231", "2025-12-31"},
		{"2026-01-01", "2026-01-01"}, // already formatted, passes through
		{"short", "short"},            // too short, passes through
	}

	for _, tt := range tests {
		got := FormatDate(tt.input)
		if got != tt.expected {
			t.Errorf("FormatDate(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestUnformatDate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2026-01-01", "20260101"},
		{"2025-12-31", "20251231"},
		{"20260101", "20260101"}, // already unformatted
	}

	for _, tt := range tests {
		got := UnformatDate(tt.input)
		if got != tt.expected {
			t.Errorf("UnformatDate(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseDailyBars(t *testing.T) {
	rows := []map[string]interface{}{
		{
			"ts_code":    "000001.SZ",
			"trade_date": "20260101",
			"open":       10.5,
			"high":       11.0,
			"low":        10.0,
			"close":      10.8,
			"pre_close":  10.3,
			"change":     0.5,
			"pct_chg":    4.85,
			"vol":        100000.0,
			"amount":     1050000.0,
		},
	}

	bars := parseDailyBars(rows)
	if len(bars) != 1 {
		t.Fatalf("expected 1 bar, got %d", len(bars))
	}
	bar := bars[0]
	if bar.TsCode != "000001.SZ" {
		t.Errorf("TsCode = %q, want 000001.SZ", bar.TsCode)
	}
	if bar.Open != 10.5 {
		t.Errorf("Open = %f, want 10.5", bar.Open)
	}
	if bar.Close != 10.8 {
		t.Errorf("Close = %f, want 10.8", bar.Close)
	}
	if bar.Vol != 100000.0 {
		t.Errorf("Vol = %f, want 100000.0", bar.Vol)
	}
}

func TestParseStockInfos(t *testing.T) {
	rows := []map[string]interface{}{
		{
			"ts_code":     "000001.SZ",
			"symbol":      "000001",
			"name":        "平安银行",
			"area":        "深圳",
			"industry":    "银行",
			"market":      "主板",
			"list_date":   "19910403",
			"list_status": "L",
		},
	}

	stocks := parseStockInfos(rows)
	if len(stocks) != 1 {
		t.Fatalf("expected 1 stock, got %d", len(stocks))
	}
	if stocks[0].Name != "平安银行" {
		t.Errorf("Name = %q, want 平安银行", stocks[0].Name)
	}
	if stocks[0].Industry != "银行" {
		t.Errorf("Industry = %q, want 银行", stocks[0].Industry)
	}
}

func TestParseCalendarDays(t *testing.T) {
	rows := []map[string]interface{}{
		{
			"exchange":      "SSE",
			"cal_date":      "20260102",
			"is_open":       1.0,
			"pretrade_date": "20251231",
		},
		{
			"exchange":      "SSE",
			"cal_date":      "20260103",
			"is_open":       0.0,
			"pretrade_date": "20260102",
		},
	}

	days := parseCalendarDays(rows)
	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(days))
	}
	if days[0].IsOpen != 1 {
		t.Errorf("first day IsOpen = %d, want 1", days[0].IsOpen)
	}
	if days[1].IsOpen != 0 {
		t.Errorf("second day IsOpen = %d, want 0", days[1].IsOpen)
	}
}

func TestGetString_NilValue(t *testing.T) {
	m := map[string]interface{}{"key": nil}
	if got := getString(m, "key"); got != "" {
		t.Errorf("getString(nil) = %q, want empty", got)
	}
	if got := getString(m, "missing"); got != "" {
		t.Errorf("getString(missing) = %q, want empty", got)
	}
}

func TestGetFloat64_TypeConversions(t *testing.T) {
	m := map[string]interface{}{
		"float":   42.5,
		"int":     int(10),
		"int64":   int64(20),
		"nil_val": nil,
	}
	if got := getFloat64(m, "float"); got != 42.5 {
		t.Errorf("getFloat64(float) = %f, want 42.5", got)
	}
	if got := getFloat64(m, "int"); got != 10.0 {
		t.Errorf("getFloat64(int) = %f, want 10.0", got)
	}
	if got := getFloat64(m, "int64"); got != 20.0 {
		t.Errorf("getFloat64(int64) = %f, want 20.0", got)
	}
	if got := getFloat64(m, "nil_val"); got != 0 {
		t.Errorf("getFloat64(nil) = %f, want 0", got)
	}
	if got := getFloat64(m, "missing"); got != 0 {
		t.Errorf("getFloat64(missing) = %f, want 0", got)
	}
}
