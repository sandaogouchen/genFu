package ruleengine

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Engine is the central rule-engine orchestrator. It dispatches position
// snapshots to registered StrategyEvaluator implementations, checks conditions,
// enforces cooldowns, and resolves conflicts.
type Engine struct {
	evaluators map[string]StrategyEvaluator
	store      RuleStore
	tracker    PositionTracker
	resolver   ConflictResolver
	cooldowns  map[string]time.Time // key: "ruleID:symbol" -> last trigger time
	cooldown   time.Duration
	mu         sync.RWMutex
}

// EngineOption is a functional option for configuring the Engine.
type EngineOption func(*Engine)

// WithCooldown sets the minimum duration between successive triggers of the
// same rule for the same symbol.
func WithCooldown(d time.Duration) EngineOption {
	return func(e *Engine) {
		e.cooldown = d
	}
}

// WithConflictResolver overrides the default conflict resolver.
func WithConflictResolver(r ConflictResolver) EngineOption {
	return func(e *Engine) {
		e.resolver = r
	}
}

// NewEngine creates a rule engine with the given store, tracker, and options.
func NewEngine(store RuleStore, tracker PositionTracker, opts ...EngineOption) *Engine {
	e := &Engine{
		evaluators: make(map[string]StrategyEvaluator),
		store:      store,
		tracker:    tracker,
		resolver:   NewDefaultConflictResolver(),
		cooldowns:  make(map[string]time.Time),
		cooldown:   0, // no cooldown by default
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Tracker returns the PositionTracker used by this engine. This allows callers
// (such as the tool layer) to access snapshot data without holding a separate
// reference to the tracker.
func (e *Engine) Tracker() PositionTracker {
	return e.tracker
}

// RegisterStrategy registers a StrategyEvaluator so the engine can dispatch
// rules that reference its StrategyID.
func (e *Engine) RegisterStrategy(evaluator StrategyEvaluator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evaluators[evaluator.StrategyID()] = evaluator
}

// CheckPosition evaluates all rules applicable to the given position snapshot.
//
// Steps:
//  1. Load rules for position from the store.
//  2. Filter to enabled rules only.
//  3. For each rule: check cooldown, evaluate conditions, dispatch to strategy,
//     and record cooldown on trigger.
//  4. Run results through the ConflictResolver.
func (e *Engine) CheckPosition(ctx context.Context, snapshot PositionSnapshot) ([]EvalResult, error) {
	rules, err := e.store.ListRulesForPosition(ctx, snapshot.AccountID, snapshot.Symbol)
	if err != nil {
		return nil, fmt.Errorf("list rules for %s/%d: %w", snapshot.Symbol, snapshot.AccountID, err)
	}

	env := buildEnvMap(snapshot)
	now := time.Now()
	var results []EvalResult

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// ---- cooldown check ----
		cooldownKey := fmt.Sprintf("%s:%s", rule.ID, snapshot.Symbol)
		if e.cooldown > 0 {
			e.mu.RLock()
			lastTrigger, hasCooldown := e.cooldowns[cooldownKey]
			e.mu.RUnlock()
			if hasCooldown && now.Sub(lastTrigger) < e.cooldown {
				continue
			}
		}

		// ---- condition check ----
		if rule.Conditions != nil {
			ok, condErr := EvaluateConditionGroup(rule.Conditions, env)
			if condErr != nil {
				return nil, fmt.Errorf("evaluate conditions for rule %s: %w", rule.ID, condErr)
			}
			if !ok {
				continue
			}
		}

		// ---- strategy dispatch ----
		e.mu.RLock()
		evaluator, found := e.evaluators[rule.StrategyID]
		e.mu.RUnlock()
		if !found {
			return nil, fmt.Errorf("no evaluator registered for strategy %q", rule.StrategyID)
		}

		result, evalErr := evaluator.Evaluate(ctx, snapshot, rule)
		if evalErr != nil {
			return nil, fmt.Errorf("evaluate rule %s with strategy %s: %w", rule.ID, rule.StrategyID, evalErr)
		}

		// Ensure RuleType is propagated from the rule definition.
		if result.RuleType == "" {
			result.RuleType = rule.Type
		}

		// ---- record cooldown on trigger ----
		if result.Triggered && e.cooldown > 0 {
			e.mu.Lock()
			e.cooldowns[cooldownKey] = now
			e.mu.Unlock()
		}

		results = append(results, result)
	}

	resolved := e.resolver.Resolve(results)
	return resolved, nil
}

// CheckAll evaluates rules for every tracked position belonging to the given
// account. It aggregates all results across positions.
func (e *Engine) CheckAll(ctx context.Context, accountID int64) ([]EvalResult, error) {
	snapshots, err := e.tracker.GetAllSnapshots(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get all snapshots for account %d: %w", accountID, err)
	}

	var allResults []EvalResult
	for _, snap := range snapshots {
		results, checkErr := e.CheckPosition(ctx, snap)
		if checkErr != nil {
			return nil, fmt.Errorf("check position %s/%d: %w", snap.Symbol, snap.AccountID, checkErr)
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// buildEnvMap constructs the environment map that condition evaluation uses.
//
// Keys:
//
//	price         - current market price
//	pnl_pct       - profit/loss percentage
//	daily_change   - daily price change percentage
//	hold_days     - whole number of days since entry
//	highest_price - highest price since entry
//	lowest_price  - lowest price since entry
//	entry_price   - original entry price
//	+ all entries from snapshot.Indicators
func buildEnvMap(snap PositionSnapshot) map[string]interface{} {
	env := map[string]interface{}{
		"price":         snap.CurrentPrice,
		"pnl_pct":       snap.PnLPct,
		"daily_change":  snap.DailyChangePct,
		"highest_price": snap.HighestPrice,
		"lowest_price":  snap.LowestPrice,
		"entry_price":   snap.EntryPrice,
	}

	// hold_days: whole number of days since entry.
	if !snap.EntryTime.IsZero() {
		holdDays := time.Since(snap.EntryTime).Hours() / 24
		env["hold_days"] = math.Floor(holdDays)
	} else {
		env["hold_days"] = float64(0)
	}

	// Merge indicator values into the environment.
	for k, v := range snap.Indicators {
		env[k] = v
	}

	return env
}
