package financial

import (
	"testing"
)

func TestStripJSONP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard JSONP",
			input:    `jsonpCallback12345({"result":[]})`,
			expected: `{"result":[]}`,
		},
		{
			name:     "JSONP with semicolon",
			input:    `callback({"data":1});`,
			expected: `{"data":1}`,
		},
		{
			name:     "plain JSON",
			input:    `{"result":[]}`,
			expected: `{"result":[]}`,
		},
		{
			name:     "JSONP with newline",
			input:    "jsonpCallback({\"data\":1})\n",
			expected: `{"data":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(stripJSONP([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("stripJSONP(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSSEResponseConversion(t *testing.T) {
	resp := &sseQueryResponse{
		Result: []sseBulletin{
			{
				SecurityCode: "600519",
				SecurityName: "贵州茅台",
				Title:        "关于重大事项的公告",
				BulletinDate: "2026-03-28",
				SSEURL:       "/disclosure/listedinfo/announcement/c/2026-03-28/600519_2026001.pdf",
			},
		},
	}
	resp.PageHelp.Total = 10
	resp.PageHelp.PageCount = 1
	resp.PageHelp.PageNo = 1
	resp.PageHelp.PageSize = 25

	result := resp.toAnnouncementResult(1)

	if result.TotalRecords != 10 {
		t.Errorf("expected TotalRecords=10, got %d", result.TotalRecords)
	}
	if len(result.Announcements) != 1 {
		t.Fatalf("expected 1 announcement, got %d", len(result.Announcements))
	}

	ann := result.Announcements[0]
	if ann.SecCode != "600519" {
		t.Errorf("expected SecCode='600519', got %q", ann.SecCode)
	}
	if ann.Column != "sse" {
		t.Errorf("expected Column='sse', got %q", ann.Column)
	}
	if ann.ExchangeName() != "上交所" {
		t.Errorf("expected ExchangeName()='上交所', got %q", ann.ExchangeName())
	}
	if ann.FormattedDate() != "2026-03-28" {
		t.Errorf("expected date '2026-03-28', got %q", ann.FormattedDate())
	}
}