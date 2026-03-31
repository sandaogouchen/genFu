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
			stop_loss, take_profit, risk_monitors, valid_until,
			trade_guide_text, trade_guide_json, trade_guide_json_v2, trade_guide_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, guide.Symbol, guide.PickID, string(buyJSON), string(sellJSON),
		guide.StopLoss, guide.TakeProfit, string(riskJSON), validUntil,
		guide.TradeGuideText, guide.TradeGuideJSON, guide.TradeGuideJSONV2, guide.TradeGuideVersion)

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
			   stop_loss, take_profit, risk_monitors, valid_until,
			   trade_guide_text, trade_guide_json, trade_guide_json_v2, trade_guide_version,
			   created_at, updated_at
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
			   stop_loss, take_profit, risk_monitors, valid_until,
			   trade_guide_text, trade_guide_json, trade_guide_json_v2, trade_guide_version,
			   created_at, updated_at
		FROM operation_guides
		WHERE id = ?
	`, id)

	return r.scanGuide(row)
}

// ListGuidesBySymbol 返回某个代码的全部历史指南（按创建时间倒序）
func (r *GuideRepository) ListGuidesBySymbol(ctx context.Context, symbol string) ([]OperationGuide, error) {
	trimmed := strings.TrimSpace(symbol)
	if trimmed == "" {
		return []OperationGuide{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, symbol, pick_id, buy_conditions, sell_conditions,
			   stop_loss, take_profit, risk_monitors, valid_until,
			   trade_guide_text, trade_guide_json, trade_guide_json_v2, trade_guide_version,
			   created_at, updated_at
		FROM operation_guides
		WHERE symbol = ?
		ORDER BY created_at DESC, id DESC
	`, trimmed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	guides := make([]OperationGuide, 0, 8)
	for rows.Next() {
		guide, err := scanGuideRow(rows)
		if err != nil {
			return nil, err
		}
		guides = append(guides, guide)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return guides, nil
}

// ListGuidesByPickID 返回某次选股的全部指南（按创建时间倒序）
func (r *GuideRepository) ListGuidesByPickID(ctx context.Context, pickID string) ([]OperationGuide, error) {
	trimmed := strings.TrimSpace(pickID)
	if trimmed == "" {
		return []OperationGuide{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, symbol, pick_id, buy_conditions, sell_conditions,
			   stop_loss, take_profit, risk_monitors, valid_until,
			   trade_guide_text, trade_guide_json, trade_guide_json_v2, trade_guide_version,
			   created_at, updated_at
		FROM operation_guides
		WHERE pick_id = ?
		ORDER BY created_at DESC, id DESC
	`, trimmed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	guides := make([]OperationGuide, 0, 8)
	for rows.Next() {
		guide, err := scanGuideRow(rows)
		if err != nil {
			return nil, err
		}
		guides = append(guides, guide)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return guides, nil
}

func (r *GuideRepository) scanGuide(row *sql.Row) (*OperationGuide, error) {
	guide, err := scanGuideRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &guide, nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanGuideRow(row rowScanner) (OperationGuide, error) {
	var guide OperationGuide
	var buyJSON, sellJSON, riskJSON sql.NullString
	var validUntil sql.NullString
	var tradeGuideText, tradeGuideJSON, tradeGuideJSONV2, tradeGuideVersion sql.NullString
	var createdAt, updatedAt sql.NullString

	err := row.Scan(
		&guide.ID, &guide.Symbol, &guide.PickID,
		&buyJSON, &sellJSON, &guide.StopLoss, &guide.TakeProfit,
		&riskJSON, &validUntil,
		&tradeGuideText, &tradeGuideJSON, &tradeGuideJSONV2, &tradeGuideVersion,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return OperationGuide{}, err
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
	if tradeGuideText.Valid {
		guide.TradeGuideText = tradeGuideText.String
	}
	if tradeGuideJSON.Valid {
		guide.TradeGuideJSON = tradeGuideJSON.String
	}
	if tradeGuideJSONV2.Valid {
		guide.TradeGuideJSONV2 = tradeGuideJSONV2.String
	}
	if tradeGuideVersion.Valid {
		guide.TradeGuideVersion = tradeGuideVersion.String
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

	return guide, nil
}
