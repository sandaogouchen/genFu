package stockpicker

import (
	"testing"
	"time"

	"genFu/internal/analyze"
)

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

func TestIsCNTradingSession(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	provider := &DefaultDataProvider{location: loc}

	cases := []struct {
		name string
		tm   time.Time
		want bool
	}{
		{
			name: "weekday_morning_open",
			tm:   time.Date(2026, 2, 25, 10, 0, 0, 0, loc),
			want: true,
		},
		{
			name: "weekday_noon_break",
			tm:   time.Date(2026, 2, 25, 12, 0, 0, 0, loc),
			want: false,
		},
		{
			name: "weekday_after_close",
			tm:   time.Date(2026, 2, 25, 15, 10, 0, 0, loc),
			want: false,
		},
		{
			name: "weekend",
			tm:   time.Date(2026, 2, 28, 10, 0, 0, 0, loc),
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := provider.isCNTradingSession(tc.tm)
			if got != tc.want {
				t.Fatalf("isCNTradingSession(%s)=%v want=%v", tc.tm.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

func TestSameCalendarDayInLocation(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	a := time.Date(2026, 2, 25, 9, 0, 0, 0, loc).UTC()
	b := time.Date(2026, 2, 25, 23, 59, 0, 0, loc)
	if !sameCalendarDayInLocation(a, b, loc) {
		t.Fatalf("expected same calendar day")
	}

	c := time.Date(2026, 2, 24, 23, 59, 0, 0, loc)
	if sameCalendarDayInLocation(c, b, loc) {
		t.Fatalf("expected different calendar day")
	}
}

func TestReportHasUsableFupanBreadth(t *testing.T) {
	report := analyze.DailyReviewReport{
		Request: map[string]interface{}{
			"data": map[string]interface{}{
				"fupan_report": map[string]interface{}{
					"up_count":   1304,
					"down_count": 3273,
				},
			},
		},
	}
	if !reportHasUsableFupanBreadth(report) {
		t.Fatalf("expected usable fupan breadth")
	}

	invalid := analyze.DailyReviewReport{
		Request: map[string]interface{}{
			"data": map[string]interface{}{
				"fupan_report": map[string]interface{}{
					"up_count":   62,
					"down_count": 17,
				},
			},
		},
	}
	if reportHasUsableFupanBreadth(invalid) {
		t.Fatalf("expected tiny breadth to be rejected")
	}
}
