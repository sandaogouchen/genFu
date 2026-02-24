package financial

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PDFAgent PDF分析Agent接口
type PDFAgent interface {
	AnalyzePDF(ctx context.Context, ann *Announcement) (*ReportSummary, error)
}

type Service struct {
	client   *CNInfoClient
	repo     *Repository
	pdfAgent PDFAgent
}

func NewService(repo *Repository, pdfAgent PDFAgent) *Service {
	return &Service{
		client:   NewCNInfoClient(),
		repo:     repo,
		pdfAgent: pdfAgent,
	}
}

// GetReportSummary 获取财报摘要（优先从缓存）
func (s *Service) GetReportSummary(ctx context.Context, symbol string) (*ReportSummary, error) {
	// 1. 尝试从缓存获取
	cached, err := s.repo.GetLatestReports(ctx, symbol, 3)
	if err != nil {
		return nil, err
	}
	if len(cached) > 0 {
		// 返回最新的一份摘要
		return s.parseReportSummary(&cached[0]), nil
	}

	// 2. 查询公告列表
	announcements, err := s.client.QueryAnnouncements(ctx, symbol, 1, 5)
	if err != nil {
		return nil, fmt.Errorf("query announcements: %w", err)
	}

	// 3. 筛选财报类型公告
	var reportAnn *Announcement
	for i := range announcements {
		if s.isFinancialReport(announcements[i].Title) {
			reportAnn = &announcements[i]
			break
		}
	}
	if reportAnn == nil {
		return nil, fmt.Errorf("no financial report found for %s", symbol)
	}

	// 4. 调用PDF Agent分析
	if s.pdfAgent == nil {
		return nil, fmt.Errorf("pdf agent not configured")
	}
	summary, err := s.pdfAgent.AnalyzePDF(ctx, reportAnn)
	if err != nil {
		return nil, fmt.Errorf("analyze pdf: %w", err)
	}

	// 5. 缓存结果
	report := &FinancialReport{
		Symbol:           reportAnn.SecCode,
		AnnouncementID:   reportAnn.ID,
		Title:            reportAnn.Title,
		ReportType:       summary.ReportType,
		AnnouncementDate: time.Unix(reportAnn.AnnouncementTime/1000, 0),
		PDFURL:           s.client.GetPDFDownloadURL(reportAnn),
		Summary:          summary.Summary,
	}
	if metricsJSON, err := json.Marshal(summary.KeyMetrics); err == nil {
		report.KeyMetrics = string(metricsJSON)
	}
	_ = s.repo.SaveReport(ctx, report)

	return summary, nil
}

// isFinancialReport 判断是否为财报类型公告
func (s *Service) isFinancialReport(title string) bool {
	keywords := []string{"年度报告", "半年度报告", "季度报告", "季报", "年报", "财务报告", "业绩��告", "业绩快报"}
	for _, kw := range keywords {
		if strings.Contains(title, kw) {
			return true
		}
	}
	return false
}

// parseReportSummary 从数据库记录解析摘要
func (s *Service) parseReportSummary(report *FinancialReport) *ReportSummary {
	summary := &ReportSummary{
		Symbol:       report.Symbol,
		ReportTitle:  report.Title,
		ReportType:   report.ReportType,
		Summary:      report.Summary,
		GeneratedAt:  report.UpdatedAt,
	}
	if report.KeyMetrics != "" {
		_ = json.Unmarshal([]byte(report.KeyMetrics), &summary.KeyMetrics)
	}
	return summary
}

// GetFinancialData 获取股票的财务数据（供选股使用）
func (s *Service) GetFinancialData(ctx context.Context, symbol string) (map[string]interface{}, error) {
	summary, err := s.GetReportSummary(ctx, symbol)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"symbol":         summary.Symbol,
		"company_name":   summary.CompanyName,
		"report_type":    summary.ReportType,
		"period":         summary.Period,
		"summary":        summary.Summary,
		"key_metrics":    summary.KeyMetrics,
		"risk_factors":   summary.RiskFactors,
		"growth_drivers": summary.GrowthDrivers,
	}, nil
}
