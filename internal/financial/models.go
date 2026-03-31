package financial

import (
	"fmt"
	"time"
)

// ============================================================
// AnnouncementQuery 公告查询请求参数
// ============================================================

// AnnouncementQuery 封装 CNInfo hisAnnouncement/query API 的全部可用参数
type AnnouncementQuery struct {
	Symbol    string // 股票代码，支持逗号分隔多只
	SearchKey string // 全文关键词（空=不按关键词过滤）
	Category  string // 公告分类码（空=全部），支持别名自动解析
	Column    string // 交易所 sse/szse/bse/空=全部
	StartDate string // 起始日期 YYYY-MM-DD（空=默认 90 天前）
	EndDate   string // 结束日期 YYYY-MM-DD（空=默认今天）
	PageNum   int    // 页码，默认 1
	PageSize  int    // 每页条数，默认 30
	SortName  string // 排序字段 date/code
	SortType  string // asc/desc
	Highlight bool   // 标题高亮
}

// Defaults 填充零值字段的默认值
func (q *AnnouncementQuery) Defaults() {
	if q.PageNum <= 0 {
		q.PageNum = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 30
	}
	if q.PageSize > 50 {
		q.PageSize = 50
	}
	if q.StartDate == "" {
		q.StartDate = time.Now().AddDate(0, -3, 0).Format("2006-01-02")
	}
	if q.EndDate == "" {
		q.EndDate = time.Now().Format("2006-01-02")
	}
}

// ============================================================
// AnnouncementResult 公告查询响应（含分页）
// ============================================================

type AnnouncementResult struct {
	Announcements []Announcement `json:"announcements"`
	TotalPages    int            `json:"total_pages"`
	TotalRecords  int            `json:"total_records"`
	CurrentPage   int            `json:"current_page"`
	HasMore       bool           `json:"has_more"`
}

// ============================================================
// Announcement 公告信息
// ============================================================

type Announcement struct {
	// === 现有字段 ===
	ID               string `json:"id"`
	SecCode          string `json:"secCode"`
	SecName          string `json:"secName"`
	Title            string `json:"announcementTitle"`
	AnnouncementTime int64  `json:"announcementTime"`
	PDFURL           string `json:"adjunctUrl"`
	PDFSize          int    `json:"adjunctSize"`
	PDFType          string `json:"adjunctType"`

	// === 新增字段 ===
	OrgID          string `json:"orgId"`
	Column         string `json:"columnCode"`
	Category       string `json:"announcementType"`
	ImportantLevel string `json:"importantLevel"`
	TitleHighlight string `json:"titleHighlight"`
}

// FormattedDate 将 Unix 毫秒时间戳格式化为 YYYY-MM-DD
func (a *Announcement) FormattedDate() string {
	if a.AnnouncementTime <= 0 {
		return ""
	}
	t := time.Unix(a.AnnouncementTime/1000, 0)
	return t.Format("2006-01-02")
}

// FullPDFURL 拼接完整的 PDF 下载链接
func (a *Announcement) FullPDFURL() string {
	if a.PDFURL == "" {
		return ""
	}
	if len(a.PDFURL) > 4 && a.PDFURL[:4] == "http" {
		return a.PDFURL
	}
	return cninfoPDFBaseURL + a.PDFURL
}

// ExchangeName 将 column code 转为中文名称
func (a *Announcement) ExchangeName() string {
	switch a.Column {
	case "sse":
		return "上交所"
	case "szse":
		return "深交所"
	case "bse":
		return "北交所"
	default:
		if a.Column == "" {
			return "未知"
		}
		return a.Column
	}
}

// CategoryName 将分类码转为中文名称
func (a *Announcement) CategoryName() string {
	if name, ok := CategoryNames[a.Category]; ok {
		return name
	}
	return a.Category
}

// PDFSizeKB 返回 PDF 大小（KB）
func (a *Announcement) PDFSizeKB() int {
	return a.PDFSize / 1024
}

// ============================================================
// FinancialReport 财报记录(数据库模型)
// ============================================================

type FinancialReport struct {
	ID               int64     `json:"id"`
	Symbol           string    `json:"symbol"`
	AnnouncementID   string    `json:"announcement_id"`
	Title            string    `json:"title"`
	ReportType       string    `json:"report_type"`
	AnnouncementDate time.Time `json:"announcement_date"`
	PDFURL           string    `json:"pdf_url"`
	Summary          string    `json:"summary"`
	KeyMetrics       string    `json:"key_metrics"` // JSON格式
	Column           string    `json:"column"`
	CategoryCode     string    `json:"category"`
	ImportantLevel   string    `json:"importance_level"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ============================================================
// ReportSummary 财报摘要(给Agent使用)
// ============================================================

type ReportSummary struct {
	Symbol        string    `json:"symbol"`
	CompanyName   string    `json:"company_name"`
	ReportTitle   string    `json:"report_title"`
	ReportType    string    `json:"report_type"`
	Period        string    `json:"period"`
	Summary       string    `json:"summary"`
	KeyMetrics    Metrics   `json:"key_metrics"`
	RiskFactors   []string  `json:"risk_factors"`
	GrowthDrivers []string  `json:"growth_drivers"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// ============================================================
// Metrics 关键财务指标
// ============================================================

type Metrics struct {
	Revenue       string `json:"revenue"`
	RevenueGrowth string `json:"revenue_growth"`
	NetProfit     string `json:"net_profit"`
	ProfitGrowth  string `json:"profit_growth"`
	GrossMargin   string `json:"gross_margin"`
	NetMargin     string `json:"net_margin"`
	ROE           string `json:"roe"`
	EPS           string `json:"eps"`
}

// ============================================================
// CNInfoQueryResponse 巨潮资讯查询响应（增强版）
// ============================================================

type CNInfoQueryResponse struct {
	Announcements  []Announcement `json:"announcements"`
	TotalPages     int            `json:"totalpages"`
	TotalRecordNum int            `json:"totalRecordNum"`
	TotalAnnouncement int         `json:"totalAnnouncement"`
	HasMore        bool           `json:"hasMore"`
}

// ToResult 将原始响应转换为标准 AnnouncementResult
func (r *CNInfoQueryResponse) ToResult(currentPage int) *AnnouncementResult {
	total := r.TotalRecordNum
	if total == 0 {
		total = r.TotalAnnouncement
	}
	return &AnnouncementResult{
		Announcements: r.Announcements,
		TotalPages:    r.TotalPages,
		TotalRecords:  total,
		CurrentPage:   currentPage,
		HasMore:       r.HasMore || currentPage < r.TotalPages,
	}
}

// ============================================================
// 输出格式化
// ============================================================

// AnnouncementListOutput 用于 Tool 输出的公告列表
type AnnouncementListOutput struct {
	Announcements []AnnouncementOutput `json:"announcements"`
	Pagination    PaginationOutput     `json:"pagination"`
}

type AnnouncementOutput struct {
	ID         string `json:"id"`
	SecCode    string `json:"sec_code"`
	SecName    string `json:"sec_name"`
	Title      string `json:"title"`
	Date       string `json:"date"`
	Exchange   string `json:"exchange"`
	Category   string `json:"category"`
	PDFURL     string `json:"pdf_url"`
	PDFSizeKB  int    `json:"pdf_size_kb"`
}

type PaginationOutput struct {
	CurrentPage  int  `json:"current_page"`
	TotalPages   int  `json:"total_pages"`
	TotalRecords int  `json:"total_records"`
	HasMore      bool `json:"has_more"`
}

// FormatForOutput 将 AnnouncementResult 转为面向 Agent 的友好输出
func (r *AnnouncementResult) FormatForOutput() *AnnouncementListOutput {
	out := &AnnouncementListOutput{
		Pagination: PaginationOutput{
			CurrentPage:  r.CurrentPage,
			TotalPages:   r.TotalPages,
			TotalRecords: r.TotalRecords,
			HasMore:      r.HasMore,
		},
	}
	for _, a := range r.Announcements {
		out.Announcements = append(out.Announcements, AnnouncementOutput{
			ID:        a.ID,
			SecCode:   a.SecCode,
			SecName:   a.SecName,
			Title:     a.Title,
			Date:      a.FormattedDate(),
			Exchange:  a.ExchangeName(),
			Category:  a.CategoryName(),
			PDFURL:    a.FullPDFURL(),
			PDFSizeKB: a.PDFSizeKB(),
		})
	}
	if out.Announcements == nil {
		out.Announcements = []AnnouncementOutput{}
	}
	return out
}

// FormatSingleAnnouncement 格式化单条公告详情
func FormatSingleAnnouncement(a *Announcement) map[string]interface{} {
	return map[string]interface{}{
		"id":               a.ID,
		"sec_code":         a.SecCode,
		"sec_name":         a.SecName,
		"title":            a.Title,
		"date":             a.FormattedDate(),
		"exchange":         a.ExchangeName(),
		"category":         a.CategoryName(),
		"pdf_url":          a.FullPDFURL(),
		"pdf_size_kb":      a.PDFSizeKB(),
		"important_level":  a.ImportantLevel,
		"title_highlight":  a.TitleHighlight,
		"org_id":           a.OrgID,
		"announcement_time": fmt.Sprintf("%d", a.AnnouncementTime),
	}
}