package financial

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"genFu/internal/db"
)

type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// GetCachedReport 获取缓存的财报摘要
func (r *Repository) GetCachedReport(ctx context.Context, announcementID string) (*FinancialReport, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, symbol, announcement_id, title, report_type, announcement_date,
			   pdf_url, summary, key_metrics, created_at, updated_at
		FROM financial_reports
		WHERE announcement_id = ?
	`, announcementID)

	var report FinancialReport
	var dateRaw, createdRaw, updatedRaw sql.NullString
	if err := row.Scan(&report.ID, &report.Symbol, &report.AnnouncementID, &report.Title,
		&report.ReportType, &dateRaw, &report.PDFURL, &report.Summary, &report.KeyMetrics,
		&createdRaw, &updatedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if parsed, ok := db.ParseTime(dateRaw); ok {
		report.AnnouncementDate = parsed
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		report.CreatedAt = parsed
	}
	if parsed, ok := db.ParseTime(updatedRaw); ok {
		report.UpdatedAt = parsed
	}
	return &report, nil
}

// SaveReport 保存财报摘要
func (r *Repository) SaveReport(ctx context.Context, report *FinancialReport) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO financial_reports (symbol, announcement_id, title, report_type,
			announcement_date, pdf_url, summary, key_metrics, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(announcement_id) DO UPDATE SET
			summary = excluded.summary,
			key_metrics = excluded.key_metrics,
			updated_at = excluded.updated_at
	`, report.Symbol, report.AnnouncementID, report.Title, report.ReportType,
		report.AnnouncementDate.Format("2006-01-02"), report.PDFURL,
		report.Summary, report.KeyMetrics, now, now)
	return err
}

// GetLatestReports 获取股票最新的财报摘要
func (r *Repository) GetLatestReports(ctx context.Context, symbol string, limit int) ([]FinancialReport, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, symbol, announcement_id, title, report_type, announcement_date,
			   pdf_url, summary, key_metrics, created_at, updated_at
		FROM financial_reports
		WHERE symbol = ? AND summary IS NOT NULL AND summary != ''
		ORDER BY announcement_date DESC
		LIMIT ?
	`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []FinancialReport
	for rows.Next() {
		var report FinancialReport
		var dateRaw, createdRaw, updatedRaw sql.NullString
		if err := rows.Scan(&report.ID, &report.Symbol, &report.AnnouncementID, &report.Title,
			&report.ReportType, &dateRaw, &report.PDFURL, &report.Summary, &report.KeyMetrics,
			&createdRaw, &updatedRaw); err != nil {
			return nil, err
		}
		if parsed, ok := db.ParseTime(dateRaw); ok {
			report.AnnouncementDate = parsed
		}
		if parsed, ok := db.ParseTime(createdRaw); ok {
			report.CreatedAt = parsed
		}
		if parsed, ok := db.ParseTime(updatedRaw); ok {
			report.UpdatedAt = parsed
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// GetReportBySymbol 获取股票最新的一份财报摘要
func (r *Repository) GetReportBySymbol(ctx context.Context, symbol string) (*FinancialReport, error) {
	reports, err := r.GetLatestReports(ctx, symbol, 1)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, nil
	}
	return &reports[0], nil
}
