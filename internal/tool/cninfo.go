package tool

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"genFu/internal/financial"
)

type CNInfoTool struct {
	client  *financial.CNInfoClient
	service *financial.Service
}

func NewCNInfoTool() *CNInfoTool {
	return &CNInfoTool{
		client: financial.NewCNInfoClient(),
	}
}

// NewCNInfoToolWithService 创建带 Service 的 CNInfoTool（支持交易所降级）
func NewCNInfoToolWithService(svc *financial.Service) *CNInfoTool {
	return &CNInfoTool{
		client:  financial.NewCNInfoClient(),
		service: svc,
	}
}

func (t CNInfoTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "cninfo",
		Description: `Search and download financial report announcements from cninfo.com.cn (巨潮资讯网).
Supports keyword search, 18 announcement categories, cross-exchange queries (SSE/SZSE/BSE), 
custom date ranges, and pagination. Can also fall back to exchange-native APIs when CNInfo is unavailable.`,
		Params: map[string]string{
			"action":          "string (search_announcements, query_announcements, get_announcement_types, get_announcement_detail, download_pdf)",
			"symbol":          "string (stock code, e.g. '600519' or '000001', supports comma-separated)",
			"keyword":         "string (search keyword, e.g. '并购重组' '业绩预告')",
			"category":        "string (category code or Chinese alias, e.g. 'category_bgcz_szsh' or '并购重组')",
			"exchange":        "string (sse/szse/bse/all, default all)",
			"start_date":      "string (YYYY-MM-DD, default 90 days ago)",
			"end_date":        "string (YYYY-MM-DD, default today)",
			"page":            "number (default 1)",
			"page_size":       "number (default 30, max 50)",
			"sort_name":       "string (sort field: 'date' or 'code', optional)",
			"sort_type":       "string (sort direction: 'asc' or 'desc', optional)",
			"pdf_url":         "string (for download_pdf action)",
			"announcement_id": "string (for get_announcement_detail action)",
		},
		Required: []string{"action"},
	}
}

func (t CNInfoTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	action, err := requireString(args, "action")
	if err != nil {
		return ToolResult{Name: "cninfo", Error: err.Error()}, err
	}

	switch strings.ToLower(action) {
	case "search_announcements":
		return t.searchAnnouncements(ctx, args)

	case "query_announcements":
		return t.queryAnnouncements(ctx, args)

	case "get_announcement_types":
		return t.getAnnouncementTypes(ctx)

	case "get_announcement_detail":
		return t.getAnnouncementDetail(ctx, args)

	case "download_pdf":
		return t.downloadPDF(ctx, args)

	default:
		return ToolResult{Name: "cninfo", Error: "unsupported_action"},
			errors.New("unsupported_action: " + action)
	}
}

// searchAnnouncements 全功能公告搜索（本 PRD 核心 Action）
func (t CNInfoTool) searchAnnouncements(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	symbol, _ := optionalString(args, "symbol")
	keyword, _ := optionalString(args, "keyword")

	// 至少需要 symbol 或 keyword 之一
	if symbol == "" && keyword == "" {
		errMsg := "search_announcements requires at least one of 'symbol' or 'keyword'"
		return ToolResult{Name: "cninfo", Error: errMsg}, errors.New(errMsg)
	}

	query := t.buildQuery(args, symbol, keyword)

	// 优先使用带降级的 Service
	var result *financial.AnnouncementResult
	var err error
	if t.service != nil {
		result, err = t.service.SearchWithFallback(ctx, query)
	} else {
		result, err = t.client.QueryAnnouncements(ctx, query)
	}

	if err != nil {
		return ToolResult{Name: "cninfo", Error: err.Error()}, err
	}

	output := result.FormatForOutput()
	if len(output.Announcements) == 0 {
		return ToolResult{Name: "cninfo", Output: map[string]interface{}{
			"announcements": []interface{}{},
			"pagination":    output.Pagination,
			"message":       "未找到匹配的公告，请尝试调整搜索条件",
		}}, nil
	}
	return ToolResult{Name: "cninfo", Output: output}, nil
}

// queryAnnouncements 增强的结构化查询（向后兼容）
func (t CNInfoTool) queryAnnouncements(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	symbol, err := requireString(args, "symbol")
	if err != nil {
		return ToolResult{Name: "cninfo", Error: err.Error()}, err
	}

	// 检查是否有新参数——如果有，使用全参数查询
	_, hasCategory := args["category"]
	_, hasExchange := args["exchange"]
	_, hasStartDate := args["start_date"]
	_, hasEndDate := args["end_date"]

	if hasCategory || hasExchange || hasStartDate || hasEndDate {
		// 使用增强查询
		query := t.buildQuery(args, symbol, "")
		result, qErr := t.client.QueryAnnouncements(ctx, query)
		if qErr != nil {
			return ToolResult{Name: "cninfo", Error: qErr.Error()}, qErr
		}
		return ToolResult{Name: "cninfo", Output: result.FormatForOutput()}, nil
	}

	// 向后兼容：仅深交所年报，近 2 年
	page, _ := optionalInt(args, "page")
	pageSize, _ := optionalInt(args, "page_size")
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	announcements, err := t.client.QueryAnnouncementsLegacy(ctx, symbol, page, pageSize)
	if err != nil {
		return ToolResult{Name: "cninfo", Error: err.Error()}, err
	}
	return ToolResult{Name: "cninfo", Output: announcements}, nil
}

// getAnnouncementTypes 返回全部公告分类类型
func (t CNInfoTool) getAnnouncementTypes(ctx context.Context) (ToolResult, error) {
	types := financial.GetAllCategoryTypes()
	return ToolResult{Name: "cninfo", Output: map[string]interface{}{
		"types":   types,
		"count":   len(types),
		"message": "可用的公告分类列表。在 search_announcements 中使用 category 参数传入 code 或中文别名进行过滤。",
	}}, nil
}

// getAnnouncementDetail 根据公告 ID 获取单条详情
func (t CNInfoTool) getAnnouncementDetail(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	annID, err := requireString(args, "announcement_id")
	if err != nil {
		return ToolResult{Name: "cninfo", Error: err.Error()}, err
	}

	// CNInfo 没有独立的单条公告查询 API
	// 这里的实现策略是返回 ID 信息 + PDF 链接提示
	return ToolResult{Name: "cninfo", Output: map[string]interface{}{
		"announcement_id": annID,
		"message":         fmt.Sprintf("公告 ID %s 的详情请通过 search_announcements 查询获取完整信息后，使用 download_pdf 下载分析。", annID),
	}}, nil
}

// downloadPDF 下载 PDF 文件
func (t CNInfoTool) downloadPDF(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	pdfURL, err := requireString(args, "pdf_url")
	if err != nil {
		return ToolResult{Name: "cninfo", Error: err.Error()}, err
	}
	data, err := t.client.DownloadPDF(ctx, pdfURL)
	if err != nil {
		return ToolResult{Name: "cninfo", Error: err.Error()}, err
	}
	return ToolResult{Name: "cninfo", Output: map[string]interface{}{
		"size":    len(data),
		"message": "PDF downloaded successfully",
	}}, nil
}

// buildQuery 从 args 构建 AnnouncementQuery
func (t CNInfoTool) buildQuery(args map[string]interface{}, symbol, keyword string) financial.AnnouncementQuery {
	category, _ := optionalString(args, "category")
	exchange, _ := optionalString(args, "exchange")
	startDate, _ := optionalString(args, "start_date")
	endDate, _ := optionalString(args, "end_date")
	sortName, _ := optionalString(args, "sort_name")
	sortType, _ := optionalString(args, "sort_type")
	page, _ := optionalInt(args, "page")
	pageSize, _ := optionalInt(args, "page_size")

	return financial.AnnouncementQuery{
		Symbol:    symbol,
		SearchKey: keyword,
		Category:  category,
		Column:    exchange,
		StartDate: startDate,
		EndDate:   endDate,
		PageNum:   page,
		PageSize:  pageSize,
		SortName:  sortName,
		SortType:  sortType,
		Highlight: true,
	}
}

// optionalString 从 args 中获取可选 string 字段
func optionalString(args map[string]interface{}, key string) (string, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}