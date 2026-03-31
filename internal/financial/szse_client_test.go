package financial

import (
	"testing"
)

func TestSZSEResponseConversion(t *testing.T) {
	resp := &szseResponse{
		Data: []szseAnnouncement{
			{
				ID:          "ann_001",
				Title:       "关于2025年度利润分配预案的公告",
				PublishDate: "2026-03-25 18:30",
				SecCode:     "000001",
				SecName:     "平安银行",
				AttachPath:  "/disc/listedinfo/content/2026/ann_001.pdf",
				AttachSize:  512000,
			},
			{
				ID:          "ann_002",
				Title:       "第十届董事会第五次会议决议公告",
				PublishDate: "2026-03-20",
				SecCode:     "000001",
				SecName:     "平安银行",
				AttachPath:  "/disc/listedinfo/content/2026/ann_002.pdf",
				AttachSize:  256000,
			},
		},
		Announce_Count: 42,
	}

	// 使用更新后的签名：toAnnouncementResult(currentPage, pageSize int)
	result := resp.toAnnouncementResult(1, 30)

	if result.TotalRecords != 42 {
		t.Errorf("expected TotalRecords=42, got %d", result.TotalRecords)
	}
	if len(result.Announcements) != 2 {
		t.Fatalf("expected 2 announcements, got %d", len(result.Announcements))
	}

	// 检查第一条公告
	ann := result.Announcements[0]
	if ann.ID != "ann_001" {
		t.Errorf("expected ID='ann_001', got %q", ann.ID)
	}
	if ann.SecCode != "000001" {
		t.Errorf("expected SecCode='000001', got %q", ann.SecCode)
	}
	if ann.Column != "szse" {
		t.Errorf("expected Column='szse', got %q", ann.Column)
	}
	if ann.ExchangeName() != "深交所" {
		t.Errorf("expected ExchangeName()='深交所', got %q", ann.ExchangeName())
	}
	if ann.FormattedDate() != "2026-03-25" {
		t.Errorf("expected date '2026-03-25', got %q", ann.FormattedDate())
	}
	if ann.PDFSizeKB() != 500 {
		t.Errorf("expected PDFSizeKB=500, got %d", ann.PDFSizeKB())
	}

	// 检查分页（42 条 / 每页 30 = 2 页）
	if result.TotalPages != 2 {
		t.Errorf("expected TotalPages=2, got %d", result.TotalPages)
	}
	if !result.HasMore {
		t.Error("expected HasMore=true for page 1 of 2")
	}
}

func TestSZSECustomPageSize(t *testing.T) {
	resp := &szseResponse{
		Data: []szseAnnouncement{
			{ID: "ann_001", SecCode: "000001", SecName: "平安银行", PublishDate: "2026-03-25"},
		},
		Announce_Count: 100,
	}

	// 使用自定义 pageSize=50
	result := resp.toAnnouncementResult(1, 50)
	if result.TotalPages != 2 { // 100/50 = 2
		t.Errorf("expected TotalPages=2 with pageSize=50, got %d", result.TotalPages)
	}

	// 使用 pageSize=10
	result2 := resp.toAnnouncementResult(1, 10)
	if result2.TotalPages != 10 { // 100/10 = 10
		t.Errorf("expected TotalPages=10 with pageSize=10, got %d", result2.TotalPages)
	}
}

func TestSZSEEmptyResponse(t *testing.T) {
	resp := &szseResponse{
		Data:           nil,
		Announce_Count: 0,
	}

	result := resp.toAnnouncementResult(1, 30)

	if result.TotalRecords != 0 {
		t.Errorf("expected TotalRecords=0, got %d", result.TotalRecords)
	}
	if len(result.Announcements) != 0 {
		t.Errorf("expected 0 announcements, got %d", len(result.Announcements))
	}
	if result.HasMore {
		t.Error("expected HasMore=false for empty result")
	}
}

func TestExchangeQueryDefaults(t *testing.T) {
	q := &ExchangeQuery{}
	q.Defaults()

	if q.PageNum != 1 {
		t.Errorf("expected default PageNum=1, got %d", q.PageNum)
	}
	if q.PageSize != 30 {
		t.Errorf("expected default PageSize=30, got %d", q.PageSize)
	}
}

func TestToExchangeQuery(t *testing.T) {
	aq := AnnouncementQuery{
		Symbol:    "600519",
		SearchKey: "年报",
		StartDate: "2026-01-01",
		EndDate:   "2026-03-31",
		PageNum:   2,
		PageSize:  20,
	}

	eq := toExchangeQuery(aq)

	if eq.Symbol != "600519" {
		t.Errorf("expected Symbol='600519', got %q", eq.Symbol)
	}
	if eq.Keyword != "年报" {
		t.Errorf("expected Keyword='年报', got %q", eq.Keyword)
	}
	if eq.StartDate != "2026-01-01" {
		t.Errorf("expected StartDate='2026-01-01', got %q", eq.StartDate)
	}
	if eq.PageNum != 2 {
		t.Errorf("expected PageNum=2, got %d", eq.PageNum)
	}
	if eq.PageSize != 20 {
		t.Errorf("expected PageSize=20, got %d", eq.PageSize)
	}
}