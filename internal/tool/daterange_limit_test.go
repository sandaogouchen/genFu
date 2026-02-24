package tool

import (
	"testing"
	"time"
)

func TestDateRangeLimit(t *testing.T) {
	tests := []struct {
		name          string
		start         string
		end           string
		klt           int
		expectLimited bool
	}{
		{
			name:          "日线-无限制-默认1年",
			start:         "",
			end:           "",
			klt:           101,
			expectLimited: true,
		},
		{
			name:          "日线-超过1年-应被限制",
			start:         "2020-01-01",
			end:           "2026-01-01",
			klt:           101,
			expectLimited: true,
		},
		{
			name:          "分钟线-超过30天-应被限制",
			start:         "2025-01-01",
			end:           "2026-01-01",
			klt:           1,
			expectLimited: true,
		},
		{
			name:          "周线-超过2年-应被限制",
			start:         "2020-01-01",
			end:           "2026-01-01",
			klt:           102,
			expectLimited: true,
		},
		{
			name:          "月线-超过5年-应被限制",
			start:         "2010-01-01",
			end:           "2026-01-01",
			klt:           103,
			expectLimited: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startTime, endTime, err := parseAndValidateDateRange(tt.start, tt.end, tt.klt, 0)
			if err != nil {
				t.Fatalf("parseAndValidateDateRange failed: %v", err)
			}

			duration := endTime.Sub(startTime)

			// 根据K线类型检查限制
			var maxDuration time.Duration
			switch tt.klt {
			case 1, 5, 15, 30, 60:
				maxDuration = 30 * 24 * time.Hour
			case 101:
				maxDuration = 365 * 24 * time.Hour
			case 102:
				maxDuration = 2 * 365 * 24 * time.Hour
			case 103:
				maxDuration = 5 * 365 * 24 * time.Hour
			}

			t.Logf("K线类型: %d", tt.klt)
			t.Logf("开始时间: %v", startTime)
			t.Logf("结束时间: %v", endTime)
			t.Logf("时间跨度: %v", duration)
			t.Logf("最大允许: %v", maxDuration)

			if tt.expectLimited && duration > maxDuration {
				t.Errorf("时间范围 %v 超过最大限制 %v", duration, maxDuration)
			}

			if duration <= 0 {
				t.Error("时间范围无效，开始时间应在结束时间之前")
			}
		})
	}
}

func TestFundDateRangeLimit(t *testing.T) {
	tests := []struct {
		name          string
		start         string
		end           string
		expectLimited bool
	}{
		{
			name:          "无限制-默认1年",
			start:         "",
			end:           "",
			expectLimited: true,
		},
		{
			name:          "超过1年-应被限制",
			start:         "2020-01-01",
			end:           "2026-01-01",
			expectLimited: true,
		},
		{
			name:          "正常范围-不被限制",
			start:         "2025-01-01",
			end:           "2026-01-01",
			expectLimited: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseAndValidateFundDateRange(tt.start, tt.end)
			if err != nil {
				t.Fatalf("parseAndValidateFundDateRange failed: %v", err)
			}

			startTime, _ := time.Parse("2006-01-02", start)
			endTime, _ := time.Parse("2006-01-02", end)
			duration := endTime.Sub(startTime)

			maxDuration := 365 * 24 * time.Hour

			t.Logf("开始时间: %v", start)
			t.Logf("结束时间: %v", end)
			t.Logf("时间跨度: %v", duration)
			t.Logf("最大允许: %v", maxDuration)

			if tt.expectLimited {
				if duration > maxDuration {
					t.Errorf("时间范围 %v 超过最大限制 %v", duration, maxDuration)
				}
			}
		})
	}
}
