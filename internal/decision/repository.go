package decision

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"genFu/internal/db"
	"genFu/internal/trade_signal"
)

type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) CreateRun(ctx context.Context, accountID int64, decisionID string, request DecisionRequest, decision trade_signal.DecisionOutput, budget RiskBudget) (int64, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return 0, errors.New("decision_repository_not_initialized")
	}
	reqJSON, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}
	decisionJSON, err := json.Marshal(decision)
	if err != nil {
		return 0, err
	}
	budgetJSON, err := json.Marshal(budget)
	if err != nil {
		return 0, err
	}
	row := r.db.QueryRowContext(ctx, `
		insert into decision_runs(
			decision_id, account_id, request_json, decision_json, risk_budget_json, status
		)
		values (?, ?, ?, ?, ?, ?)
		returning id
	`, decisionID, accountID, reqJSON, decisionJSON, budgetJSON, "running")
	var runID int64
	if err := row.Scan(&runID); err != nil {
		return 0, err
	}
	return runID, nil
}

func (r *Repository) SaveOrders(ctx context.Context, runID int64, orders []GuardedOrder) error {
	if r == nil || r.db == nil || r.db.DB == nil {
		return errors.New("decision_repository_not_initialized")
	}
	if runID == 0 {
		return errors.New("invalid_run_id")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from execution_orders where run_id = ?`, runID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, order := range orders {
		notional := order.Notional
		if notional <= 0 {
			notional = order.Quantity * order.Price
		}
		var tradeID interface{}
		if order.TradeID > 0 {
			tradeID = order.TradeID
		} else {
			tradeID = nil
		}
		if _, err := tx.ExecContext(ctx, `
			insert into execution_orders(
				run_id, order_id, symbol, name, asset_type, action, quantity, price, notional, confidence,
				planner_reason, guard_status, guard_reason, execution_status, trade_id, execution_error
			)
			values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			runID,
			order.OrderID,
			order.Symbol,
			order.Name,
			order.AssetType,
			order.Action,
			order.Quantity,
			order.Price,
			notional,
			order.Confidence,
			order.PlanningReason,
			order.GuardStatus,
			order.GuardReason,
			order.ExecutionStatus,
			tradeID,
			order.ExecutionError,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) SaveReview(ctx context.Context, runID int64, review *PostTradeReview) error {
	if r == nil || r.db == nil || r.db.DB == nil {
		return errors.New("decision_repository_not_initialized")
	}
	if runID == 0 {
		return errors.New("invalid_run_id")
	}
	if review == nil {
		return nil
	}
	reviewJSON, err := json.Marshal(review)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from post_trade_reviews where run_id = ?`, runID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		insert into post_trade_reviews(run_id, summary, review_json)
		values (?, ?, ?)
	`, runID, review.Summary, reviewJSON); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *Repository) FinalizeRun(ctx context.Context, runID int64, status string) error {
	if r == nil || r.db == nil || r.db.DB == nil {
		return errors.New("decision_repository_not_initialized")
	}
	if runID == 0 {
		return errors.New("invalid_run_id")
	}
	if status == "" {
		return errors.New("empty_status")
	}
	res, err := r.db.ExecContext(ctx, `
		update decision_runs
		set status = ?, updated_at = CURRENT_TIMESTAMP
		where id = ?
	`, status, runID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("decision_run_not_found: %d", runID)
	}
	return nil
}

func (r *Repository) GetReviewByRunID(ctx context.Context, runID int64) (*PostTradeReview, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return nil, errors.New("decision_repository_not_initialized")
	}
	row := r.db.QueryRowContext(ctx, `
		select review_json
		from post_trade_reviews
		where run_id = ?
		order by id desc
		limit 1
	`, runID)
	var raw sql.NullString
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	var review PostTradeReview
	if err := json.Unmarshal([]byte(raw.String), &review); err != nil {
		return nil, err
	}
	return &review, nil
}
