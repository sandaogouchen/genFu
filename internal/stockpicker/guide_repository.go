package stockpicker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"genFu/internal/db"
)

type GuideRepository struct {
	db *db.DB
}

func NewGuideRepository(database *db.DB) *GuideRepository {
	return &GuideRepository{db: database}
}

// SaveGuide 保存操作指南
func (r *GuideRepository) SaveGuide(ctx context.Context, guide *OperationGuide) error {
	buyJSON, _ := json.Marshal(guide.BuyConditions)
	sellJSON, _ := json.Marshal(guide.SellConditions)
	riskJSON, _ := json.Marshal(guide.RiskMonitors)

	var validUntil sql.NullString
	if guide.ValidUntil != nil {
		validUntil.String = guide.ValidUntil.Format("2006-01-02")
		validUntil.Valid = true
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO operation_guides (symbol, pick_id, buy_conditions, sell_conditions,
			stop_loss, take_profit, risk_monitors, valid_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, guide.Symbol, guide.PickID, string(buyJSON), string(sellJSON),
		guide.StopLoss, guide.TakeProfit, string(riskJSON), validUntil)

	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()
	guide.ID = id
	return nil
}

// GetLatestGuide 获取股票最新的操作指南
func (r *GuideRepository) GetLatestGuide(ctx context.Context, symbol string) (*OperationGuide, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, symbol, pick_id, buy_conditions, sell_conditions,
			   stop_loss, take_profit, risk_monitors, valid_until, created_at, updated_at
		FROM operation_guides
		WHERE symbol = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, symbol)

	return r.scanGuide(row)
}

// GetGuideByID 通过ID获取操作指南
func (r *GuideRepository) GetGuideByID(ctx context.Context, id int64) (*OperationGuide, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, symbol, pick_id, buy_conditions, sell_conditions,
			   stop_loss, take_profit, risk_monitors, valid_until, created_at, updated_at
		FROM operation_guides
		WHERE id = ?
	`, id)

	return r.scanGuide(row)
}

func (r *GuideRepository) scanGuide(row *sql.Row) (*OperationGuide, error) {
	var guide OperationGuide
	var buyJSON, sellJSON, riskJSON sql.NullString
	var validUntil sql.NullString
	var createdAt, updatedAt sql.NullString

	err := row.Scan(
		&guide.ID, &guide.Symbol, &guide.PickID,
		&buyJSON, &sellJSON, &guide.StopLoss, &guide.TakeProfit,
		&riskJSON, &validUntil, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if buyJSON.Valid {
		json.Unmarshal([]byte(buyJSON.String), &guide.BuyConditions)
	}
	if sellJSON.Valid {
		json.Unmarshal([]byte(sellJSON.String), &guide.SellConditions)
	}
	if riskJSON.Valid {
		json.Unmarshal([]byte(riskJSON.String), &guide.RiskMonitors)
	}
	if validUntil.Valid {
		t, _ := time.Parse("2006-01-02", validUntil.String)
		guide.ValidUntil = &t
	}
	if createdAt.Valid {
		guide.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt.String)
	}
	if updatedAt.Valid {
		guide.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt.String)
	}

	return &guide, nil
}
