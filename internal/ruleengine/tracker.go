package ruleengine

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PositionTracker manages real-time position tracking state used by the rule
// engine to evaluate stop-loss and take-profit conditions.
type PositionTracker interface {
	// UpdatePrice records a new market price for a tracked position, updating
	// the highest/lowest watermarks and last-seen price.
	UpdatePrice(ctx context.Context, accountID int64, symbol string, price float64, ts time.Time) error

	// GetSnapshot returns the current tracking snapshot for a single position.
	GetSnapshot(ctx context.Context, accountID int64, symbol string) (PositionSnapshot, error)

	// GetAllSnapshots returns tracking snapshots for every position belonging
	// to the given account.
	GetAllSnapshots(ctx context.Context, accountID int64) ([]PositionSnapshot, error)

	// ResetTracking removes all tracking data for the specified position.
	ResetTracking(ctx context.Context, accountID int64, symbol string) error
}

// SQLitePositionTracker is a PositionTracker backed by a SQLite database.
type SQLitePositionTracker struct {
	db *sql.DB
}

// NewSQLitePositionTracker creates a new tracker. The caller is responsible for
// creating the sl_tp_position_tracking table before use.
func NewSQLitePositionTracker(db *sql.DB) *SQLitePositionTracker {
	return &SQLitePositionTracker{db: db}
}

// UpdatePrice performs an UPSERT into sl_tp_position_tracking. On conflict it
// updates highest_price = MAX(existing, new), lowest_price = MIN(existing, new),
// last_price, and last_update.
func (t *SQLitePositionTracker) UpdatePrice(ctx context.Context, accountID int64, symbol string, price float64, ts time.Time) error {
	const query = `
INSERT INTO sl_tp_position_tracking (
    account_id, symbol, entry_price, highest_price, lowest_price,
    last_price, entry_time, last_update
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, symbol) DO UPDATE SET
    highest_price = MAX(highest_price, excluded.highest_price),
    lowest_price  = MIN(lowest_price, excluded.lowest_price),
    last_price    = excluded.last_price,
    last_update   = excluded.last_update
`
	_, err := t.db.ExecContext(ctx, query,
		accountID, symbol,
		price, // entry_price (only used on first insert)
		price, // highest_price
		price, // lowest_price
		price, // last_price
		ts,    // entry_time (only used on first insert)
		ts,    // last_update
	)
	if err != nil {
		return fmt.Errorf("update price for %s/%d: %w", symbol, accountID, err)
	}
	return nil
}

// GetSnapshot retrieves the current tracking row and builds a PositionSnapshot.
// PnLPct is calculated as (last_price - entry_price) / entry_price.
func (t *SQLitePositionTracker) GetSnapshot(ctx context.Context, accountID int64, symbol string) (PositionSnapshot, error) {
	const query = `
SELECT account_id, symbol, entry_price, highest_price, lowest_price,
       last_price, entry_time, last_update
FROM sl_tp_position_tracking
WHERE account_id = ? AND symbol = ?
`
	var snap PositionSnapshot
	var entryTime, lastUpdate string

	err := t.db.QueryRowContext(ctx, query, accountID, symbol).Scan(
		&snap.AccountID,
		&snap.Symbol,
		&snap.EntryPrice,
		&snap.HighestPrice,
		&snap.LowestPrice,
		&snap.CurrentPrice, // last_price maps to CurrentPrice
		&entryTime,
		&lastUpdate,
	)
	if err != nil {
		return PositionSnapshot{}, fmt.Errorf("get snapshot %s/%d: %w", symbol, accountID, err)
	}

	snap.EntryTime, err = parseTime(entryTime)
	if err != nil {
		return PositionSnapshot{}, fmt.Errorf("parse entry_time: %w", err)
	}
	snap.LastUpdate, err = parseTime(lastUpdate)
	if err != nil {
		return PositionSnapshot{}, fmt.Errorf("parse last_update: %w", err)
	}

	// MarketPrice mirrors CurrentPrice so strategies that reference either field
	// get the same value.
	snap.MarketPrice = snap.CurrentPrice

	if snap.EntryPrice != 0 {
		snap.PnLPct = (snap.CurrentPrice - snap.EntryPrice) / snap.EntryPrice
	}
	snap.Indicators = make(map[string]float64)

	return snap, nil
}

// GetAllSnapshots returns every tracked position for the given account.
func (t *SQLitePositionTracker) GetAllSnapshots(ctx context.Context, accountID int64) ([]PositionSnapshot, error) {
	const query = `
SELECT account_id, symbol, entry_price, highest_price, lowest_price,
       last_price, entry_time, last_update
FROM sl_tp_position_tracking
WHERE account_id = ?
`
	rows, err := t.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("get all snapshots for account %d: %w", accountID, err)
	}
	defer rows.Close()

	var snapshots []PositionSnapshot
	for rows.Next() {
		var snap PositionSnapshot
		var entryTime, lastUpdate string

		if err := rows.Scan(
			&snap.AccountID,
			&snap.Symbol,
			&snap.EntryPrice,
			&snap.HighestPrice,
			&snap.LowestPrice,
			&snap.CurrentPrice,
			&entryTime,
			&lastUpdate,
		); err != nil {
			return nil, fmt.Errorf("scan snapshot row: %w", err)
		}

		snap.EntryTime, err = parseTime(entryTime)
		if err != nil {
			return nil, fmt.Errorf("parse entry_time: %w", err)
		}
		snap.LastUpdate, err = parseTime(lastUpdate)
		if err != nil {
			return nil, fmt.Errorf("parse last_update: %w", err)
		}

		// MarketPrice mirrors CurrentPrice so strategies that reference either
		// field get the same value.
		snap.MarketPrice = snap.CurrentPrice

		if snap.EntryPrice != 0 {
			snap.PnLPct = (snap.CurrentPrice - snap.EntryPrice) / snap.EntryPrice
		}
		snap.Indicators = make(map[string]float64)

		snapshots = append(snapshots, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate snapshot rows: %w", err)
	}

	return snapshots, nil
}

// ResetTracking deletes the tracking row for the specified position.
func (t *SQLitePositionTracker) ResetTracking(ctx context.Context, accountID int64, symbol string) error {
	const query = `DELETE FROM sl_tp_position_tracking WHERE account_id = ? AND symbol = ?`
	_, err := t.db.ExecContext(ctx, query, accountID, symbol)
	if err != nil {
		return fmt.Errorf("reset tracking %s/%d: %w", symbol, accountID, err)
	}
	return nil
}

// parseTime tries common time layouts that SQLite may store.
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		time.DateTime,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time %q", s)
}
