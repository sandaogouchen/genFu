package analyze

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"genFu/internal/db"
)

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateReport(ctx context.Context, req AnalyzeRequest, resp AnalyzeResponse) (AnalyzeReport, error) {
	if r == nil {
		return AnalyzeReport{}, errors.New("repository_not_initialized")
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return AnalyzeReport{}, err
	}
	stepsJSON, err := json.Marshal(resp.Steps)
	if err != nil {
		return AnalyzeReport{}, err
	}
	row := r.db.QueryRowContext(ctx, `
		insert into analyze_reports(report_type, symbol, name, request, steps, summary)
		values (?, ?, ?, ?, ?, ?)
		returning id, report_type, symbol, name, title, request, steps, summary, created_at
	`, resp.Type, resp.Symbol, resp.Name, reqJSON, stepsJSON, resp.Summary)
	var report AnalyzeReport
	var reqRaw, stepsRaw []byte
	var createdRaw sql.NullString
	if err := row.Scan(&report.ID, &report.ReportType, &report.Symbol, &report.Name, &report.Title, &reqRaw, &stepsRaw, &report.Summary, &createdRaw); err != nil {
		return AnalyzeReport{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		report.CreatedAt = parsed
	}
	_ = json.Unmarshal(reqRaw, &report.Request)
	_ = json.Unmarshal(stepsRaw, &report.Steps)
	return report, nil
}

func (r *Repository) GetReport(ctx context.Context, id int64) (AnalyzeReport, error) {
	if r == nil {
		return AnalyzeReport{}, errors.New("repository_not_initialized")
	}
	row := r.db.QueryRowContext(ctx, `
		select id, report_type, symbol, name, title, request, steps, summary, created_at
		from analyze_reports
		where id = ?
	`, id)
	var report AnalyzeReport
	var reqRaw, stepsRaw []byte
	var createdRaw sql.NullString
	if err := row.Scan(&report.ID, &report.ReportType, &report.Symbol, &report.Name, &report.Title, &reqRaw, &stepsRaw, &report.Summary, &createdRaw); err != nil {
		return AnalyzeReport{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		report.CreatedAt = parsed
	}
	_ = json.Unmarshal(reqRaw, &report.Request)
	_ = json.Unmarshal(stepsRaw, &report.Steps)
	return report, nil
}

func (r *Repository) GetLatestReportByType(ctx context.Context, reportType string) (AnalyzeReport, error) {
	if r == nil {
		return AnalyzeReport{}, errors.New("repository_not_initialized")
	}
	row := r.db.QueryRowContext(ctx, `
		select id, report_type, symbol, name, title, request, steps, summary, created_at
		from analyze_reports
		where report_type = ?
		order by created_at desc
		limit 1
	`, reportType)
	var report AnalyzeReport
	var reqRaw, stepsRaw []byte
	var createdRaw sql.NullString
	if err := row.Scan(&report.ID, &report.ReportType, &report.Symbol, &report.Name, &report.Title, &reqRaw, &stepsRaw, &report.Summary, &createdRaw); err != nil {
		return AnalyzeReport{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		report.CreatedAt = parsed
	}
	_ = json.Unmarshal(reqRaw, &report.Request)
	_ = json.Unmarshal(stepsRaw, &report.Steps)
	return report, nil
}

// DailyReviewReport 每日复盘报告（使用map存储原始数据）
type DailyReviewReport struct {
	ID         int64
	ReportType string
	Name       string
	Request    map[string]interface{}
	Summary    string
	CreatedAt  time.Time
}

// GetLatestDailyReviewReport 获取最新的每日复盘报告（原始map格式）
func (r *Repository) GetLatestDailyReviewReport(ctx context.Context) (DailyReviewReport, error) {
	if r == nil {
		return DailyReviewReport{}, errors.New("repository_not_initialized")
	}
	row := r.db.QueryRowContext(ctx, `
		select id, report_type, name, request, summary, created_at
		from analyze_reports
		where report_type = 'daily_review'
		order by created_at desc
		limit 1
	`)
	var report DailyReviewReport
	var reqRaw []byte
	var createdRaw sql.NullString
	if err := row.Scan(&report.ID, &report.ReportType, &report.Name, &reqRaw, &report.Summary, &createdRaw); err != nil {
		return DailyReviewReport{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		report.CreatedAt = parsed
	}
	_ = json.Unmarshal(reqRaw, &report.Request)
	return report, nil
}

func (r *Repository) CreateDailyReviewReport(ctx context.Context, name string, payload map[string]interface{}, summary string) (int64, error) {
	if r == nil {
		return 0, errors.New("repository_not_initialized")
	}
	reqJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	stepsJSON, err := json.Marshal([]AnalyzeStep{})
	if err != nil {
		return 0, err
	}
	row := r.db.QueryRowContext(ctx, `
		insert into analyze_reports(report_type, symbol, name, request, steps, summary)
		values (?, ?, ?, ?, ?, ?)
		returning id
	`, "daily_review", "market", name, reqJSON, stepsJSON, summary)
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) CreateNextOpenGuideReport(ctx context.Context, name string, payload map[string]interface{}, summary string) (int64, error) {
	if r == nil {
		return 0, errors.New("repository_not_initialized")
	}
	reqJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	stepsJSON, err := json.Marshal([]AnalyzeStep{})
	if err != nil {
		return 0, err
	}
	row := r.db.QueryRowContext(ctx, `
		insert into analyze_reports(report_type, symbol, name, request, steps, summary)
		values (?, ?, ?, ?, ?, ?)
		returning id
	`, "next_open_guide", "portfolio", name, reqJSON, stepsJSON, summary)
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// ReportListItem is a simplified report structure for list display
type ReportListItem struct {
	ID         int64     `json:"id"`
	ReportType string    `json:"report_type"`
	Symbol     string    `json:"symbol"`
	Name       string    `json:"name"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"created_at"`
}

// ListReports retrieves a paginated list of reports with optional filtering
func (r *Repository) ListReports(ctx context.Context, reportType, search string, page, pageSize int) ([]ReportListItem, error) {
	if r == nil {
		return nil, errors.New("repository_not_initialized")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := `select id, report_type, symbol, name, title, created_at from analyze_reports`
	var args []interface{}
	var conditions []string

	if reportType != "" {
		conditions = append(conditions, "report_type = ?")
		args = append(args, reportType)
	}
	if search != "" {
		conditions = append(conditions, "(title LIKE ? OR name LIKE ?)")
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if len(conditions) > 0 {
		query += " where " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " and " + conditions[i]
		}
	}

	query += " order by created_at desc limit ? offset ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ReportListItem
	for rows.Next() {
		var item ReportListItem
		var createdRaw sql.NullString
		if err := rows.Scan(&item.ID, &item.ReportType, &item.Symbol, &item.Name, &item.Title, &createdRaw); err != nil {
			return nil, err
		}
		if parsed, ok := db.ParseTime(createdRaw); ok {
			item.CreatedAt = parsed
		}
		items = append(items, item)
	}

	if items == nil {
		items = []ReportListItem{}
	}

	return items, nil
}

// CountReports returns the total number of reports matching the filter criteria
func (r *Repository) CountReports(ctx context.Context, reportType, search string) (int, error) {
	if r == nil {
		return 0, errors.New("repository_not_initialized")
	}

	query := `select count(*) from analyze_reports`
	var args []interface{}
	var conditions []string

	if reportType != "" {
		conditions = append(conditions, "report_type = ?")
		args = append(args, reportType)
	}
	if search != "" {
		conditions = append(conditions, "(title LIKE ? OR name LIKE ?)")
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if len(conditions) > 0 {
		query += " where " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " and " + conditions[i]
		}
	}

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateReportTitle updates the title of a report
func (r *Repository) UpdateReportTitle(ctx context.Context, id int64, title string) error {
	if r == nil {
		return errors.New("repository_not_initialized")
	}

	_, err := r.db.ExecContext(ctx, `
		update analyze_reports
		set title = ?
		where id = ?
	`, title, id)

	return err
}
