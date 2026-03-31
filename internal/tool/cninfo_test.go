package tool

import (
	"context"
	"testing"

	"genFu/internal/financial"
)

func TestCNInfoToolSpec(t *testing.T) {
	tool := NewCNInfoTool()
	spec := tool.Spec()

	if spec.Name != "cninfo" {
		t.Errorf("expected tool name 'cninfo', got %q", spec.Name)
	}

	// 检查必需参数
	if len(spec.Required) != 1 || spec.Required[0] != "action" {
		t.Errorf("expected required=['action'], got %v", spec.Required)
	}

	// 检查关键参数存在
	requiredParams := []string{"action", "symbol", "keyword", "category", "exchange",
		"start_date", "end_date", "page", "page_size", "sort_name", "sort_type",
		"pdf_url", "announcement_id"}
	for _, p := range requiredParams {
		if _, ok := spec.Params[p]; !ok {
			t.Errorf("missing param %q in spec", p)
		}
	}
}

func TestSearchAnnouncementsValidation(t *testing.T) {
	tool := NewCNInfoTool()
	ctx := context.Background()

	// 测试：symbol 和 keyword 都为空时应报错
	args := map[string]interface{}{
		"action": "search_announcements",
	}
	result, err := tool.Execute(ctx, args)
	if err == nil {
		t.Error("expected error when both symbol and keyword are empty")
	}
	if result.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestQueryAnnouncementsBackwardCompat(t *testing.T) {
	tool := NewCNInfoTool()
	ctx := context.Background()

	// 缺少 symbol 应报错
	args := map[string]interface{}{
		"action": "query_announcements",
	}
	_, err := tool.Execute(ctx, args)
	if err == nil {
		t.Error("expected error when symbol is missing for query_announcements")
	}
}

func TestGetAnnouncementTypes(t *testing.T) {
	tool := NewCNInfoTool()
	ctx := context.Background()

	args := map[string]interface{}{
		"action": "get_announcement_types",
	}
	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output, ok := result.Output.(map[string]interface{})
	if !ok {
		t.Fatal("expected map output")
	}

	types, ok := output["types"].([]map[string]string)
	if !ok {
		t.Fatal("expected []map[string]string for types")
	}

	if len(types) != 18 {
		t.Errorf("expected 18 announcement types, got %d", len(types))
	}

	// 验证每个类型都有 code 和 name
	for _, typ := range types {
		if typ["code"] == "" {
			t.Error("found type with empty code")
		}
		if typ["name"] == "" {
			t.Error("found type with empty name")
		}
	}
}

func TestBuildQuery(t *testing.T) {
	tool := NewCNInfoTool()

	args := map[string]interface{}{
		"action":     "search_announcements",
		"symbol":     "600519",
		"keyword":    "并购重组",
		"category":   "并购",
		"exchange":   "all",
		"start_date": "2026-01-01",
		"end_date":   "2026-03-31",
		"page":       2,
		"page_size":  20,
		"sort_name":  "date",
		"sort_type":  "desc",
	}

	query := tool.buildQuery(args, "600519", "并购重组")

	if query.Symbol != "600519" {
		t.Errorf("expected symbol '600519', got %q", query.Symbol)
	}
	if query.SearchKey != "并购重组" {
		t.Errorf("expected keyword '并购重组', got %q", query.SearchKey)
	}
	if query.Category != "并购" {
		t.Errorf("expected category '并购', got %q", query.Category)
	}
	if query.Column != "all" {
		t.Errorf("expected exchange 'all', got %q", query.Column)
	}
	if query.StartDate != "2026-01-01" {
		t.Errorf("expected start_date '2026-01-01', got %q", query.StartDate)
	}
	if query.PageNum != 2 {
		t.Errorf("expected page 2, got %d", query.PageNum)
	}
	if query.PageSize != 20 {
		t.Errorf("expected page_size 20, got %d", query.PageSize)
	}
	if query.SortName != "date" {
		t.Errorf("expected sort_name 'date', got %q", query.SortName)
	}
	if query.SortType != "desc" {
		t.Errorf("expected sort_type 'desc', got %q", query.SortType)
	}
}

func TestBuildQuerySortDefaults(t *testing.T) {
	tool := NewCNInfoTool()

	// 不传 sort_name/sort_type 时应为空
	args := map[string]interface{}{
		"symbol": "600519",
	}
	query := tool.buildQuery(args, "600519", "")

	if query.SortName != "" {
		t.Errorf("expected empty sort_name, got %q", query.SortName)
	}
	if query.SortType != "" {
		t.Errorf("expected empty sort_type, got %q", query.SortType)
	}
}

func TestCategoryResolution(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"并购重组", "category_bgcz_szsh"},
		{"年报", "category_ndbg_szsh"},
		{"定增", "category_zpfx_szsh"},
		{"category_bgcz_szsh", "category_bgcz_szsh"},
		{"", ""},
		{"季报", "category_yjdbg_szsh;category_sjdbg_szsh"},
	}

	for _, tt := range tests {
		result := financial.ResolveCategoryCode(tt.input)
		if result != tt.expected {
			t.Errorf("ResolveCategoryCode(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestAnnouncementFormatting(t *testing.T) {
	ann := &financial.Announcement{
		ID:               "12345",
		SecCode:          "600519",
		SecName:          "贵州茅台",
		Title:            "年度报告",
		AnnouncementTime: 1711584000000, // 2024-03-28
		PDFURL:           "finalpage/2024-03-28/1234.PDF",
		PDFSize:          1310720, // ~1280 KB
		Column:           "sse",
		Category:         "category_ndbg_szsh",
	}

	if ann.FormattedDate() == "" {
		t.Error("FormattedDate should not be empty")
	}
	if ann.ExchangeName() != "上交所" {
		t.Errorf("expected '上交所', got %q", ann.ExchangeName())
	}
	if ann.CategoryName() != "年度报告" {
		t.Errorf("expected '年度报告', got %q", ann.CategoryName())
	}
	if ann.PDFSizeKB() != 1280 {
		t.Errorf("expected 1280 KB, got %d", ann.PDFSizeKB())
	}
	fullURL := ann.FullPDFURL()
	if fullURL == "" || !containsSubstring(fullURL, "static.cninfo.com.cn") {
		t.Errorf("unexpected full PDF URL: %q", fullURL)
	}
}

func TestExchangeStockDetection(t *testing.T) {
	tests := []struct {
		symbol string
		isSSE  bool
	}{
		{"600519", true},
		{"688001", true},
		{"000001", false},
		{"300001", false},
		{"830001", false},
	}

	for _, tt := range tests {
		// 使用 internal 包的导出测试函数
		if financial.IsSSEStockExported(tt.symbol) != tt.isSSE {
			t.Errorf("isSSEStock(%q) = %v, want %v", tt.symbol, !tt.isSSE, tt.isSSE)
		}
	}
}

// helpers

func findInString(s, sub string) (int, bool) {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i, true
		}
	}
	return -1, false
}

func containsSubstring(s, sub string) bool {
	_, found := findInString(s, sub)
	return found
}