package ruleengine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RuleStore defines persistence operations for rules and trigger events.
type RuleStore interface {
	// CreateRule persists a new rule. If r.ID is empty a UUID is generated.
	CreateRule(ctx context.Context, r *Rule) error
	// UpdateRule modifies an existing rule in place.
	UpdateRule(ctx context.Context, r *Rule) error
	// DeleteRule removes a rule by its ID.
	DeleteRule(ctx context.Context, ruleID string) error
	// GetRule retrieves a single rule by ID.
	GetRule(ctx context.Context, ruleID string) (*Rule, error)
	// ListRules returns rules matching the supplied filter.
	ListRules(ctx context.Context, f RuleFilter) ([]Rule, error)
	// ListRulesForPosition returns all rules that apply to a specific position
	// (matching account_id AND either the symbol list contains the symbol or scope_level='account').
	ListRulesForPosition(ctx context.Context, accountID int64, symbol string) ([]Rule, error)
	// LogTrigger records a trigger event.
	LogTrigger(ctx context.Context, evt *TriggerEvent) error
	// ListTriggers returns trigger events matching the supplied filter.
	ListTriggers(ctx context.Context, f TriggerFilter) ([]TriggerEvent, error)
}

// ---------------------------------------------------------------------------
// SQLiteRuleStore
// ---------------------------------------------------------------------------

// SQLiteRuleStore implements RuleStore backed by a SQLite database.
type SQLiteRuleStore struct {
	db *sql.DB
}

// NewSQLiteRuleStore creates a new SQLiteRuleStore.
func NewSQLiteRuleStore(db *sql.DB) *SQLiteRuleStore {
	return &SQLiteRuleStore{db: db}
}

// CreateRule persists a new rule. A UUID is generated when r.ID is empty.
func (s *SQLiteRuleStore) CreateRule(ctx context.Context, r *Rule) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now

	paramsJSON, err := marshalJSONField(r.Params)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal params: %w", err)
	}

	conditionsJSON, err := json.Marshal(r.Conditions)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal conditions: %w", err)
	}

	actionJSON, err := json.Marshal(r.Action)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal action: %w", err)
	}

	symbolsJSON, err := json.Marshal(r.Scope.Symbols)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal symbols: %w", err)
	}

	const query = `
		INSERT INTO sl_tp_rules (
			id, name, type, strategy_id, params,
			scope_level, account_id, symbols,
			priority, enabled, conditions, action,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		r.ID, r.Name, r.Type, r.StrategyID, string(paramsJSON),
		r.Scope.Level, r.Scope.AccountID, string(symbolsJSON),
		r.Priority, r.Enabled, string(conditionsJSON), string(actionJSON),
		r.CreatedAt.Format(time.RFC3339), r.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("ruleengine: create rule %s: %w", r.ID, err)
	}
	return nil
}

// UpdateRule modifies an existing rule and bumps UpdatedAt.
func (s *SQLiteRuleStore) UpdateRule(ctx context.Context, r *Rule) error {
	r.UpdatedAt = time.Now().UTC()

	paramsJSON, err := marshalJSONField(r.Params)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal params: %w", err)
	}

	conditionsJSON, err := json.Marshal(r.Conditions)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal conditions: %w", err)
	}

	actionJSON, err := json.Marshal(r.Action)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal action: %w", err)
	}

	symbolsJSON, err := json.Marshal(r.Scope.Symbols)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal symbols: %w", err)
	}

	const query = `
		UPDATE sl_tp_rules SET
			name = ?, type = ?, strategy_id = ?, params = ?,
			scope_level = ?, account_id = ?, symbols = ?,
			priority = ?, enabled = ?, conditions = ?, action = ?,
			updated_at = ?
		WHERE id = ?
	`

	res, err := s.db.ExecContext(ctx, query,
		r.Name, r.Type, r.StrategyID, string(paramsJSON),
		r.Scope.Level, r.Scope.AccountID, string(symbolsJSON),
		r.Priority, r.Enabled, string(conditionsJSON), string(actionJSON),
		r.UpdatedAt.Format(time.RFC3339),
		r.ID,
	)
	if err != nil {
		return fmt.Errorf("ruleengine: update rule %s: %w", r.ID, err)
	}
	return checkRowsAffected(res, "rule", r.ID)
}

// DeleteRule removes a rule by its ID.
func (s *SQLiteRuleStore) DeleteRule(ctx context.Context, ruleID string) error {
	const query = `DELETE FROM sl_tp_rules WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, ruleID)
	if err != nil {
		return fmt.Errorf("ruleengine: delete rule %s: %w", ruleID, err)
	}
	return checkRowsAffected(res, "rule", ruleID)
}

// GetRule retrieves a single rule by its ID.
func (s *SQLiteRuleStore) GetRule(ctx context.Context, ruleID string) (*Rule, error) {
	const query = `
		SELECT id, name, type, strategy_id, params,
		       scope_level, account_id, symbols,
		       priority, enabled, conditions, action,
		       created_at, updated_at
		FROM sl_tp_rules
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, ruleID)
	r, err := scanRule(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ruleengine: rule %s not found", ruleID)
		}
		return nil, fmt.Errorf("ruleengine: get rule %s: %w", ruleID, err)
	}
	return r, nil
}

// ListRules returns rules matching the supplied filter.
func (s *SQLiteRuleStore) ListRules(ctx context.Context, f RuleFilter) ([]Rule, error) {
	clauses := make([]string, 0, 4)
	args := make([]interface{}, 0, 4)

	if f.AccountID != 0 {
		clauses = append(clauses, "account_id = ?")
		args = append(args, f.AccountID)
	}
	if f.RuleType != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, f.RuleType)
	}
	if f.EnabledOnly {
		clauses = append(clauses, "enabled = 1")
	}
	if len(f.Symbols) > 0 {
		placeholders := make([]string, len(f.Symbols))
		for i, sym := range f.Symbols {
			placeholders[i] = "symbols LIKE ?"
			args = append(args, "%"+sym+"%")
		}
		clauses = append(clauses, "("+strings.Join(placeholders, " OR ")+")")
	}

	query := `
		SELECT id, name, type, strategy_id, params,
		       scope_level, account_id, symbols,
		       priority, enabled, conditions, action,
		       created_at, updated_at
		FROM sl_tp_rules
	`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY priority DESC, created_at ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ruleengine: list rules: %w", err)
	}
	defer rows.Close()

	return collectRules(rows)
}

// ListRulesForPosition returns rules that apply to the given account + symbol.
// A rule matches when its account_id equals the provided accountID AND either
// the scope_level is 'account' (applies to all symbols) or the symbols JSON
// array contains the requested symbol.
func (s *SQLiteRuleStore) ListRulesForPosition(ctx context.Context, accountID int64, symbol string) ([]Rule, error) {
	const query = `
		SELECT id, name, type, strategy_id, params,
		       scope_level, account_id, symbols,
		       priority, enabled, conditions, action,
		       created_at, updated_at
		FROM sl_tp_rules
		WHERE account_id = ?
		  AND enabled = 1
		  AND (scope_level = 'account' OR symbols LIKE ?)
		ORDER BY priority DESC, created_at ASC
	`

	symbolPattern := "%" + symbol + "%"
	rows, err := s.db.QueryContext(ctx, query, accountID, symbolPattern)
	if err != nil {
		return nil, fmt.Errorf("ruleengine: list rules for position account=%d symbol=%s: %w", accountID, symbol, err)
	}
	defer rows.Close()

	return collectRules(rows)
}

// LogTrigger persists a trigger event into sl_tp_trigger_events.
func (s *SQLiteRuleStore) LogTrigger(ctx context.Context, evt *TriggerEvent) error {
	actionJSON, err := json.Marshal(evt.Action)
	if err != nil {
		return fmt.Errorf("ruleengine: marshal trigger action: %w", err)
	}

	var executedAt *string
	if evt.ExecutedAt != nil {
		t := evt.ExecutedAt.Format(time.RFC3339)
		executedAt = &t
	}

	const query = `
		INSERT INTO sl_tp_trigger_events (
			rule_id, account_id, symbol,
			trigger_price, market_price, pnl_pct,
			action, status, trade_id, reason,
			triggered_at, executed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	res, err := s.db.ExecContext(ctx, query,
		evt.RuleID, evt.AccountID, evt.Symbol,
		evt.TriggerPrice, evt.MarketPrice, evt.PnLPct,
		string(actionJSON), evt.Status, evt.TradeID, evt.Reason,
		evt.TriggeredAt.Format(time.RFC3339), executedAt,
	)
	if err != nil {
		return fmt.Errorf("ruleengine: log trigger: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("ruleengine: log trigger last insert id: %w", err)
	}
	evt.ID = id
	return nil
}

// ListTriggers returns trigger events matching the supplied filter.
func (s *SQLiteRuleStore) ListTriggers(ctx context.Context, f TriggerFilter) ([]TriggerEvent, error) {
	clauses := make([]string, 0, 6)
	args := make([]interface{}, 0, 6)

	if f.AccountID != 0 {
		clauses = append(clauses, "account_id = ?")
		args = append(args, f.AccountID)
	}
	if f.Symbol != "" {
		clauses = append(clauses, "symbol = ?")
		args = append(args, f.Symbol)
	}
	if f.RuleID != "" {
		clauses = append(clauses, "rule_id = ?")
		args = append(args, f.RuleID)
	}
	if f.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, f.Status)
	}
	if f.Since != nil {
		clauses = append(clauses, "triggered_at >= ?")
		args = append(args, f.Since.Format(time.RFC3339))
	}
	if f.Until != nil {
		clauses = append(clauses, "triggered_at <= ?")
		args = append(args, f.Until.Format(time.RFC3339))
	}

	query := `
		SELECT id, rule_id, account_id, symbol,
		       trigger_price, market_price, pnl_pct,
		       action, status, trade_id, reason,
		       triggered_at, executed_at
		FROM sl_tp_trigger_events
	`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY triggered_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ruleengine: list triggers: %w", err)
	}
	defer rows.Close()

	var events []TriggerEvent
	for rows.Next() {
		evt, err := scanTriggerEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("ruleengine: scan trigger event: %w", err)
		}
		events = append(events, *evt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ruleengine: iterate trigger events: %w", err)
	}
	return events, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanRule reads a single rule row from any scanner.
func scanRule(sc scanner) (*Rule, error) {
	var (
		r                Rule
		paramsStr        string
		scopeLevel       string
		accountID        int64
		symbolsStr       string
		conditionsStr    string
		actionStr        string
		createdAtStr     string
		updatedAtStr     string
	)

	err := sc.Scan(
		&r.ID, &r.Name, &r.Type, &r.StrategyID, &paramsStr,
		&scopeLevel, &accountID, &symbolsStr,
		&r.Priority, &r.Enabled, &conditionsStr, &actionStr,
		&createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	r.Params = json.RawMessage(paramsStr)
	r.Scope.Level = scopeLevel
	r.Scope.AccountID = accountID

	if symbolsStr != "" && symbolsStr != "null" {
		if err := json.Unmarshal([]byte(symbolsStr), &r.Scope.Symbols); err != nil {
			return nil, fmt.Errorf("unmarshal symbols: %w", err)
		}
	}

	if conditionsStr != "" && conditionsStr != "null" {
		var cg ConditionGroup
		if err := json.Unmarshal([]byte(conditionsStr), &cg); err != nil {
			return nil, fmt.Errorf("unmarshal conditions: %w", err)
		}
		r.Conditions = &cg
	}

	if err := json.Unmarshal([]byte(actionStr), &r.Action); err != nil {
		return nil, fmt.Errorf("unmarshal action: %w", err)
	}

	if r.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr); err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	if r.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr); err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &r, nil
}

// collectRules iterates sql.Rows and returns a slice of Rules.
func collectRules(rows *sql.Rows) ([]Rule, error) {
	var rules []Rule
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("ruleengine: scan rule: %w", err)
		}
		rules = append(rules, *r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ruleengine: iterate rules: %w", err)
	}
	return rules, nil
}

// scanTriggerEvent reads a single trigger event from a scanner.
func scanTriggerEvent(sc scanner) (*TriggerEvent, error) {
	var (
		evt           TriggerEvent
		actionStr     string
		triggeredStr  string
		executedStr   *string
	)

	err := sc.Scan(
		&evt.ID, &evt.RuleID, &evt.AccountID, &evt.Symbol,
		&evt.TriggerPrice, &evt.MarketPrice, &evt.PnLPct,
		&actionStr, &evt.Status, &evt.TradeID, &evt.Reason,
		&triggeredStr, &executedStr,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(actionStr), &evt.Action); err != nil {
		return nil, fmt.Errorf("unmarshal trigger action: %w", err)
	}

	if evt.TriggeredAt, err = time.Parse(time.RFC3339, triggeredStr); err != nil {
		return nil, fmt.Errorf("parse triggered_at: %w", err)
	}

	if executedStr != nil {
		t, err := time.Parse(time.RFC3339, *executedStr)
		if err != nil {
			return nil, fmt.Errorf("parse executed_at: %w", err)
		}
		evt.ExecutedAt = &t
	}

	return &evt, nil
}

// marshalJSONField returns raw JSON bytes. If the input is already a non-nil
// json.RawMessage it is returned as-is; otherwise it is marshalled.
func marshalJSONField(raw json.RawMessage) ([]byte, error) {
	if len(raw) > 0 {
		return []byte(raw), nil
	}
	return []byte("{}"), nil
}

// checkRowsAffected verifies that at least one row was affected by an
// UPDATE or DELETE statement.
func checkRowsAffected(res sql.Result, entity, id string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ruleengine: rows affected for %s %s: %w", entity, id, err)
	}
	if n == 0 {
		return fmt.Errorf("ruleengine: %s %s not found", entity, id)
	}
	return nil
}
