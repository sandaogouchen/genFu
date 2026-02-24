package financial

import "time"

// Announcement 公告信息
type Announcement struct {
	ID               string `json:"id"`               // 公告ID
	SecCode          string `json:"secCode"`          // 股票代码
	SecName          string `json:"secName"`          // 公司名称
	Title            string `json:"announcementTitle"` // 公告标题
	AnnouncementTime int64  `json:"announcementTime"`  // 公告时间戳
	PDFURL           string `json:"adjunctUrl"`       // PDF相对路径
	PDFSize          int    `json:"adjunctSize"`      // PDF大小
	PDFType          string `json:"adjunctType"`      // 文件类型
}

// FinancialReport 财报记录(数据库模型)
type FinancialReport struct {
	ID              int64      `json:"id"`
	Symbol          string     `json:"symbol"`
	AnnouncementID  string     `json:"announcement_id"`
	Title           string     `json:"title"`
	ReportType      string     `json:"report_type"`
	AnnouncementDate time.Time `json:"announcement_date"`
	PDFURL          string     `json:"pdf_url"`
	Summary         string     `json:"summary"`
	KeyMetrics      string     `json:"key_metrics"`      // JSON格式
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ReportSummary 财报摘要(给Agent使用)
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

// Metrics 关键财务指标
type Metrics struct {
	Revenue       string `json:"revenue"`        // 营收
	RevenueGrowth string `json:"revenue_growth"` // 营收增长
	NetProfit     string `json:"net_profit"`     // 净利润
	ProfitGrowth  string `json:"profit_growth"`  // 利润增长
	GrossMargin   string `json:"gross_margin"`   // 毛利率
	NetMargin     string `json:"net_margin"`     // 净利率
	ROE           string `json:"roe"`            // 净资产收益率
	EPS           string `json:"eps"`            // 每股收益
}

// CNInfoQueryResponse 巨潮资讯查询响应
type CNInfoQueryResponse struct {
	Announcements []Announcement `json:"announcements"`
}
