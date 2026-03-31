package stockpicker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"genFu/internal/db"
)

type RunRepository struct {
	db *db.DB
}

func NewRunRepository(database *db.DB) *RunRepository {
	return &RunRepository{db: database}
}

type StockPickRunSnapshot struct {
	Request       interface{}
	MarketData    interface{}
	Regime        interface{}
	Routing       interface{}
	CandidatePool interface{}
	Analysis      interface{}
	PortfolioFit  interface{}
	TradeGuides   interface{}
	Warnings      interface{}
	Status        string
	ErrorMessage  string
}

type StockPickRunRecord struct {
	PickID            string    `json:"pick_id"`
	RequestJSON       string    `json:"request_json"`
	MarketDataJSON    string    `json:"market_data_json"`
	RegimeJSON        string    `json:"regime_json"`
	RoutingJSON       string    `json:"routing_json"`
	CandidatePoolJSON string    `json:"candidate_pool_json"`
	AnalysisJSON      string    `json:"analysis_json"`
	PortfolioFitJSON  string    `json:"portfolio_fit_json"`
	TradeGuidesJSON   string    `json:"trade_guides_json"`
	WarningsJSON      string    `json:"warnings_json"`
	Status            string    `json:"status"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type StockPickRunSummary struct {
	PickID        string      `json:"pick_id"`
	Status        string      `json:"status"`
	ErrorMessage  string      `json:"error_message,omitempty"`
	Regime        interface{} `json:"regime,omitempty"`
	Routing       interface{} `json:"routing,omitempty"`
	CandidatePool interface{} `json:"candidate_pool,omitempty"`
	PortfolioFit  interface{} `json:"portfolio_fit,omitempty"`
	Warnings      interface{} `json:"warnings,omitempty"`
	UpdatedAt     time.Time   `json:"updated_at,omitempty"`
}

func (r *RunRepository) SaveByPickID(ctx context.Context, pickID string, snapshot StockPickRunSnapshot) error {
	if r == nil || r.db == nil {
		return errors.New("run_repository_not_initialized")
	}
	pickID = strings.TrimSpace(pickID)
	if pickID == "" {
		return errors.New("missing_pick_id")
	}

	status := strings.TrimSpace(snapshot.Status)
	if status == "" {
		status = "completed"
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO stock_pick_runs (
			pick_id,
			request_json,
			market_data_json,
			regime_json,
			routing_json,
			candidate_pool_json,
			analysis_json,
			portfolio_fit_json,
			trade_guides_json,
			warnings_json,
			status,
			error_message
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pick_id) DO UPDATE SET
			request_json=excluded.request_json,
			market_data_json=excluded.market_data_json,
			regime_json=excluded.regime_json,
			routing_json=excluded.routing_json,
			candidate_pool_json=excluded.candidate_pool_json,
			analysis_json=excluded.analysis_json,
			portfolio_fit_json=excluded.portfolio_fit_json,
			trade_guides_json=excluded.trade_guides_json,
			warnings_json=excluded.warnings_json,
			status=excluded.status,
			error_message=excluded.error_message,
			updated_at=CURRENT_TIMESTAMP
	`,
		pickID,
		marshalJSONOrDefault(snapshot.Request, `{}`),
		marshalJSONOrDefault(snapshot.MarketData, `{}`),
		marshalJSONOrDefault(snapshot.Regime, `{}`),
		marshalJSONOrDefault(snapshot.Routing, `{}`),
		marshalJSONOrDefault(snapshot.CandidatePool, `{}`),
		marshalJSONOrDefault(snapshot.Analysis, `{}`),
		marshalJSONOrDefault(snapshot.PortfolioFit, `{}`),
		marshalJSONOrDefault(snapshot.TradeGuides, `{}`),
		marshalJSONOrDefault(snapshot.Warnings, `[]`),
		status,
		strings.TrimSpace(snapshot.ErrorMessage),
	)
	return err
}

func (r *RunRepository) GetByPickID(ctx context.Context, pickID string) (*StockPickRunRecord, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("run_repository_not_initialized")
	}
	pickID = strings.TrimSpace(pickID)
	if pickID == "" {
		return nil, nil
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			pick_id,
			request_json,
			market_data_json,
			regime_json,
			routing_json,
			candidate_pool_json,
			analysis_json,
			portfolio_fit_json,
			trade_guides_json,
			warnings_json,
			status,
			error_message,
			created_at,
			updated_at
		FROM stock_pick_runs
		WHERE pick_id = ?
		LIMIT 1
	`, pickID)

	var record StockPickRunRecord
	var errMsg sql.NullString
	var createdAtRaw, updatedAtRaw string
	if err := row.Scan(
		&record.PickID,
		&record.RequestJSON,
		&record.MarketDataJSON,
		&record.RegimeJSON,
		&record.RoutingJSON,
		&record.CandidatePoolJSON,
		&record.AnalysisJSON,
		&record.PortfolioFitJSON,
		&record.TradeGuidesJSON,
		&record.WarningsJSON,
		&record.Status,
		&errMsg,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if errMsg.Valid {
		record.ErrorMessage = errMsg.String
	}
	record.CreatedAt = parseSQLiteTime(createdAtRaw)
	record.UpdatedAt = parseSQLiteTime(updatedAtRaw)
	return &record, nil
}

func (r *RunRepository) BuildSummary(record *StockPickRunRecord) *StockPickRunSummary {
	if record == nil {
		return nil
	}
	return &StockPickRunSummary{
		PickID:        record.PickID,
		Status:        record.Status,
		ErrorMessage:  strings.TrimSpace(record.ErrorMessage),
		Regime:        decodeJSONLoose(record.RegimeJSON),
		Routing:       decodeJSONLoose(record.RoutingJSON),
		CandidatePool: decodeJSONLoose(record.CandidatePoolJSON),
		PortfolioFit:  decodeJSONLoose(record.PortfolioFitJSON),
		Warnings:      decodeJSONLoose(record.WarningsJSON),
		UpdatedAt:     record.UpdatedAt,
	}
}

func marshalJSONOrDefault(v interface{}, fallback string) string {
	if v == nil {
		return fallback
	}
	raw, err := json.Marshal(v)
	if err != nil || len(raw) == 0 {
		return fallback
	}
	return string(raw)
}

func decodeJSONLoose(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return raw
	}
	return out
}

func parseSQLiteTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts
		}
	}
	return time.Time{}
}
