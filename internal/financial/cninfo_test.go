package financial

import (
	"context"
	"testing"
)

// 确保 context 正常使用（消除 import 用于后续集成测试）
var _ = context.Background

func TestResolveCategoryCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// 标准分类码
		{"category_ndbg_szsh", "category_ndbg_szsh"},
		{"category_bgcz_szsh", "category_bgcz_szsh"},

		// 中文别名
		{"年报", "category_ndbg_szsh"},
		{"年度报告", "category_ndbg_szsh"},
		{"半年报", "category_bndbg_szsh"},
		{"并购", "category_bgcz_szsh"},
		{"重组", "category_bgcz_szsh"},
		{"并购重组", "category_bgcz_szsh"},
		{"定增", "category_zpfx_szsh"},
		{"分红", "category_fhps_szsh"},
		{"业绩预告", "category_yjyg_szsh"},
		{"业绩快报", "category_yjkb_szsh"},
		{"股权激励", "category_gqbd_szsh"},
		{"股东大会", "category_gddh_szsh"},

		// 多分类
		{"季报", "category_yjdbg_szsh;category_sjdbg_szsh"},

		// 空值
		{"", ""},

		// 未知值
		{"unknown_category", "unknown_category"},
	}

	for _, tt := range tests {
		result := ResolveCategoryCode(tt.input)
		if result != tt.expected {
			t.Errorf("ResolveCategoryCode(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetAllCategoryTypes(t *testing.T) {
	types := GetAllCategoryTypes()

	if len(types) != 18 {
		t.Errorf("expected 18 category types, got %d", len(types))
	}

	// 验证第一个是年度报告
	if types[0]["code"] != "category_ndbg_szsh" {
		t.Errorf("first category should be 年度报告, got %s", types[0]["code"])
	}
	if types[0]["name"] != "年度报告" {
		t.Errorf("first category name should be 年度报告, got %s", types[0]["name"])
	}

	// 验证所有条目都有必填字段
	for i, typ := range types {
		if typ["code"] == "" {
			t.Errorf("category %d: missing code", i)
		}
		if typ["name"] == "" {
			t.Errorf("category %d: missing name", i)
		}
	}
}

func TestAnnouncementQueryDefaults(t *testing.T) {
	q := &AnnouncementQuery{}
	q.Defaults()

	if q.PageNum != 1 {
		t.Errorf("expected default PageNum=1, got %d", q.PageNum)
	}
	if q.PageSize != 30 {
		t.Errorf("expected default PageSize=30, got %d", q.PageSize)
	}
	if q.StartDate == "" {
		t.Error("expected non-empty default StartDate")
	}
	if q.EndDate == "" {
		t.Error("expected non-empty default EndDate")
	}

	// 超出最大值
	q2 := &AnnouncementQuery{PageSize: 100}
	q2.Defaults()
	if q2.PageSize != 50 {
		t.Errorf("expected capped PageSize=50, got %d", q2.PageSize)
	}
}

func TestAnnouncementFormatting(t *testing.T) {
	ann := &Announcement{
		ID:               "12345",
		SecCode:          "600519",
		SecName:          "贵州茅台",
		Title:            "关于重大资产收购的公告",
		AnnouncementTime: 1711584000000,
		PDFURL:           "finalpage/2024-03-28/1234.PDF",
		PDFSize:          1310720,
		Column:           "sse",
		Category:         "category_bgcz_szsh",
	}

	// FormattedDate
	date := ann.FormattedDate()
	if date == "" {
		t.Error("FormattedDate should not be empty for valid timestamp")
	}

	// FullPDFURL
	fullURL := ann.FullPDFURL()
	if fullURL == "" {
		t.Error("FullPDFURL should not be empty")
	}
	if fullURL[:4] != "http" {
		t.Errorf("FullPDFURL should start with http, got %q", fullURL)
	}

	// ExchangeName
	if ann.ExchangeName() != "上交所" {
		t.Errorf("expected '上交所', got %q", ann.ExchangeName())
	}

	// CategoryName
	if ann.CategoryName() != "并购重组" {
		t.Errorf("expected '并购重组', got %q", ann.CategoryName())
	}

	// PDFSizeKB
	if ann.PDFSizeKB() != 1280 {
		t.Errorf("expected 1280 KB, got %d", ann.PDFSizeKB())
	}

	// 空值测试
	emptyAnn := &Announcement{}
	if emptyAnn.FormattedDate() != "" {
		t.Error("FormattedDate should be empty for zero timestamp")
	}
	if emptyAnn.FullPDFURL() != "" {
		t.Error("FullPDFURL should be empty for empty PDFURL")
	}
	if emptyAnn.ExchangeName() != "未知" {
		t.Errorf("expected '未知', got %q", emptyAnn.ExchangeName())
	}
}

func TestCNInfoQueryResponseToResult(t *testing.T) {
	resp := &CNInfoQueryResponse{
		Announcements:  []Announcement{{ID: "1"}, {ID: "2"}},
		TotalPages:     5,
		TotalRecordNum: 142,
		HasMore:        true,
	}

	result := resp.ToResult(1)
	if result.CurrentPage != 1 {
		t.Errorf("expected CurrentPage=1, got %d", result.CurrentPage)
	}
	if result.TotalPages != 5 {
		t.Errorf("expected TotalPages=5, got %d", result.TotalPages)
	}
	if result.TotalRecords != 142 {
		t.Errorf("expected TotalRecords=142, got %d", result.TotalRecords)
	}
	if !result.HasMore {
		t.Error("expected HasMore=true")
	}
	if len(result.Announcements) != 2 {
		t.Errorf("expected 2 announcements, got %d", len(result.Announcements))
	}
}

func TestResolveColumn(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sse", "sse"},
		{"SSE", "sse"},
		{"szse", "szse"},
		{"bse", "bse"},
		{"all", ""},
		{"", ""},
		{" sse ", "sse"},
	}

	for _, tt := range tests {
		result := resolveColumn(tt.input)
		if result != tt.expected {
			t.Errorf("resolveColumn(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// IsSSEStockExported 导出 isSSEStock 用于跨包测试（tool 包需要）
func IsSSEStockExported(symbol string) bool {
	return isSSEStock(symbol)
}

func TestIsSSEStock(t *testing.T) {
	tests := []struct {
		symbol string
		expect bool
	}{
		{"600519", true},
		{"688001", true},
		{"000001", false},
		{"300001", false},
		{"830001", false},
		{"", false},
	}

	for _, tt := range tests {
		if isSSEStock(tt.symbol) != tt.expect {
			t.Errorf("isSSEStock(%q) = %v, want %v", tt.symbol, !tt.expect, tt.expect)
		}
	}
}

func TestIsSZSEStock(t *testing.T) {
	tests := []struct {
		symbol string
		expect bool
	}{
		{"000001", true},
		{"300001", true},
		{"600519", false},
		{"830001", false},
	}

	for _, tt := range tests {
		if isSZSEStock(tt.symbol) != tt.expect {
			t.Errorf("isSZSEStock(%q) = %v, want %v", tt.symbol, !tt.expect, tt.expect)
		}
	}
}