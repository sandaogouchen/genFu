package stockpicker

import "testing"

func TestParseRiseDownFromText(t *testing.T) {
	up, down, ok := parseRiseDownFromText("个股涨跌图 上涨1304(28%) 停牌12(0%) 下跌3273(71%) 来源：同花顺")
	if !ok {
		t.Fatalf("expected parse success")
	}
	if up != 1304 || down != 3273 {
		t.Fatalf("unexpected result up=%d down=%d", up, down)
	}
}

func TestParseRiseDownFromTextRejectTinyCounts(t *testing.T) {
	up, down, ok := parseRiseDownFromText("上涨66(83%) 下跌13(17%)")
	if ok {
		t.Fatalf("expected parse rejected, got up=%d down=%d", up, down)
	}
}

func TestExtractFupanRiseDownRejectTinyUpDown(t *testing.T) {
	payload := map[string]interface{}{
		"fupan_report": map[string]interface{}{
			"up_count":   66,
			"down_count": 13,
		},
	}
	_, _, ok := extractFupanRiseDown(payload)
	if ok {
		t.Fatalf("expected tiny fupan up/down rejected")
	}
}
