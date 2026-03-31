package ruleengine

import (
	"encoding/json"
	"time"
)

// RuleType constants define the category of a rule.
const (
	RuleTypeStopLoss   = "stop_loss"
	RuleTypeTakeProfit = "take_profit"
)

// Rule represents a stop-loss or take-profit rule configuration.
type Rule struct {
	ID         string          `json:"id" yaml:"id"`
	Name       string          `json:"name" yaml:"name"`
	Type       string          `json:"type" yaml:"type"`
	RuleType   string          `json:"rule_type" yaml:"rule_type"`
	StrategyID string          `json:"strategy_id" yaml:"strategy_id"`
	Params     json.RawMessage `json:"params" yaml:"params"`
	Scope      RuleScope       `json:"scope" yaml:"scope"`
	Priority   int             `json:"priority" yaml:"priority"`
	Enabled    bool            `json:"enabled" yaml:"enabled"`
	Conditions *ConditionGroup `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Action     RuleAction      `json:"action" yaml:"action"`
	CreatedAt  time.Time       `json:"created_at" yaml:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" yaml:"updated_at"`
}

// RuleScope defines the applicability of a rule.
type RuleScope struct {
	Level     string   `json:"level" yaml:"level"`
	AccountID int64    `json:"account_id" yaml:"account_id"`
	Symbols   []string `json:"symbols,omitempty" yaml:"symbols,omitempty"`
}

// ConditionGroup is a recursive logical grouping of conditions.
// Groups is a slice of pointers so that recursive evaluation can pass
// each sub-group directly to EvaluateConditionGroup(*ConditionGroup, ...).
type ConditionGroup struct {
	Operator   string            `json:"operator" yaml:"operator"`
	Conditions []Condition       `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Groups     []*ConditionGroup `json:"groups,omitempty" yaml:"groups,omitempty"`
}

// Condition represents a single evaluable predicate.
type Condition struct {
	Field    string      `json:"field" yaml:"field"`
	Operator string      `json:"operator" yaml:"operator"`
	Value    interface{} `json:"value" yaml:"value"`
}

// RuleAction describes what to do when a rule triggers.
type RuleAction struct {
	ActionType   string  `json:"action_type" yaml:"action_type"`
	SellPercent  float64 `json:"sell_percent,omitempty" yaml:"sell_percent,omitempty"`
	SellQuantity float64 `json:"sell_quantity,omitempty" yaml:"sell_quantity,omitempty"`
	Urgency      string  `json:"urgency,omitempty" yaml:"urgency,omitempty"`
}

// TakeProfitTier represents one tier in a partial take-profit strategy.
type TakeProfitTier struct {
	TierID      string  `json:"tier_id" yaml:"tier_id"`
	TriggerPct  float64 `json:"trigger_pct" yaml:"trigger_pct"`
	SellPercent float64 `json:"sell_percent" yaml:"sell_percent"`
	Trailing    bool    `json:"trailing" yaml:"trailing"`
	TrailPct    float64 `json:"trail_pct,omitempty" yaml:"trail_pct,omitempty"`
	Executed    bool    `json:"executed" yaml:"executed"`
}

// ---------------------------------------------------------------------------
// Strategy parameter types
// ---------------------------------------------------------------------------

// FixedPctSLParams holds parameters for a fixed-percentage stop-loss strategy.
type FixedPctSLParams struct {
	ThresholdPct float64 `json:"threshold_pct" yaml:"threshold_pct"`
}

// TrailingSLParams holds parameters for a trailing stop-loss strategy.
type TrailingSLParams struct {
	TrailPct      float64 `json:"trail_pct" yaml:"trail_pct"`
	ActivationPct float64 `json:"activation_pct" yaml:"activation_pct"`
}

// ATRSLParams holds parameters for an ATR-based stop-loss strategy.
type ATRSLParams struct {
	Period     int     `json:"period" yaml:"period"`
	Multiplier float64 `json:"multiplier" yaml:"multiplier"`
}

// DailyDropSLParams holds parameters for a daily-drop stop-loss strategy.
type DailyDropSLParams struct {
	MaxDailyDropPct float64 `json:"max_daily_drop_pct" yaml:"max_daily_drop_pct"`
}

// FixedPctTPParams holds parameters for a fixed-percentage take-profit strategy.
type FixedPctTPParams struct {
	TargetPct float64 `json:"target_pct" yaml:"target_pct"`
}

// TrailingTPParams holds parameters for a trailing take-profit strategy.
type TrailingTPParams struct {
	ActivationPct float64 `json:"activation_pct" yaml:"activation_pct"`
	TrailPct      float64 `json:"trail_pct" yaml:"trail_pct"`
}

// PartialTPParams holds parameters for a partial (tiered) take-profit strategy.
type PartialTPParams struct {
	Tiers []TakeProfitTier `json:"tiers" yaml:"tiers"`
}

// ---------------------------------------------------------------------------
// Runtime / evaluation types
// ---------------------------------------------------------------------------

// PositionSnapshot captures the current state of a held position for evaluation.
type PositionSnapshot struct {
	AccountID      int64              `json:"account_id" yaml:"account_id"`
	Symbol         string             `json:"symbol" yaml:"symbol"`
	Quantity       float64            `json:"quantity" yaml:"quantity"`
	AvgCost        float64            `json:"avg_cost" yaml:"avg_cost"`
	EntryPrice     float64            `json:"entry_price" yaml:"entry_price"`
	MarketPrice    float64            `json:"market_price" yaml:"market_price"`
	CurrentPrice   float64            `json:"current_price" yaml:"current_price"`
	HighestPrice   float64            `json:"highest_price" yaml:"highest_price"`
	LowestPrice    float64            `json:"lowest_price" yaml:"lowest_price"`
	EntryTime      time.Time          `json:"entry_time" yaml:"entry_time"`
	LastUpdate     time.Time          `json:"last_update" yaml:"last_update"`
	PnLPct         float64            `json:"pnl_pct" yaml:"pnl_pct"`
	DailyChange    float64            `json:"daily_change" yaml:"daily_change"`
	DailyChangePct float64            `json:"daily_change_pct" yaml:"daily_change_pct"`
	Indicators     map[string]float64 `json:"indicators,omitempty" yaml:"indicators,omitempty"`
}

// EvalResult is the outcome of evaluating a single rule against a position.
type EvalResult struct {
	RuleID       string     `json:"rule_id" yaml:"rule_id"`
	RuleName     string     `json:"rule_name" yaml:"rule_name"`
	RuleType     string     `json:"rule_type" yaml:"rule_type"`
	Symbol       string     `json:"symbol" yaml:"symbol"`
	Triggered    bool       `json:"triggered" yaml:"triggered"`
	TriggerPrice float64    `json:"trigger_price" yaml:"trigger_price"`
	MarketPrice  float64    `json:"market_price" yaml:"market_price"`
	PnLPct       float64    `json:"pnl_pct" yaml:"pnl_pct"`
	Reason       string     `json:"reason" yaml:"reason"`
	Action       RuleAction `json:"action" yaml:"action"`
	Priority     int        `json:"priority" yaml:"priority"`
	EvalTime     time.Time  `json:"eval_time" yaml:"eval_time"`
}

// TriggerSignal is emitted when a rule fires and an order should be placed.
type TriggerSignal struct {
	SignalID     string     `json:"signal_id"`
	AccountID    int64      `json:"account_id"`
	Symbol       string     `json:"symbol"`
	RuleID       string     `json:"rule_id"`
	RuleName     string     `json:"rule_name"`
	RuleType     string     `json:"rule_type"`
	StrategyID   string     `json:"strategy_id"`
	Action       RuleAction `json:"action"`
	TriggerPrice float64    `json:"trigger_price"`
	MarketPrice  float64    `json:"market_price"`
	PnLPct       float64    `json:"pnl_pct"`
	Reason       string     `json:"reason"`
	Urgency      string     `json:"urgency"`
	TriggeredAt  time.Time  `json:"triggered_at"`
}

// TriggerEvent is the persisted record of a trigger and its execution status.
type TriggerEvent struct {
	ID           int64      `json:"id"`
	RuleID       string     `json:"rule_id"`
	AccountID    int64      `json:"account_id"`
	Symbol       string     `json:"symbol"`
	TriggerPrice float64    `json:"trigger_price"`
	MarketPrice  float64    `json:"market_price"`
	PnLPct       float64    `json:"pnl_pct"`
	Action       RuleAction `json:"action"`
	Status       string     `json:"status"`
	TradeID      *int64     `json:"trade_id,omitempty"`
	Reason       string     `json:"reason"`
	TriggeredAt  time.Time  `json:"triggered_at"`
	ExecutedAt   *time.Time `json:"executed_at,omitempty"`
}

// ---------------------------------------------------------------------------
// Query filter types
// ---------------------------------------------------------------------------

// RuleFilter is used to query rules from the store.
type RuleFilter struct {
	AccountID   int64    `json:"account_id"`
	Symbols     []string `json:"symbols,omitempty"`
	RuleType    string   `json:"rule_type,omitempty"`
	EnabledOnly bool     `json:"enabled_only,omitempty"`
}

// TriggerFilter is used to query trigger events from the store.
type TriggerFilter struct {
	AccountID int64      `json:"account_id"`
	Symbol    string     `json:"symbol,omitempty"`
	RuleID    string     `json:"rule_id,omitempty"`
	Status    string     `json:"status,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
}
