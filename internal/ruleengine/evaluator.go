package ruleengine

import "context"

// Evaluator is the top-level interface for evaluating a set of rules against a
// position snapshot. Implementations orchestrate strategy dispatch, condition
// checking, and conflict resolution.
type Evaluator interface {
	// Evaluate runs every rule in the slice against the given snapshot and
	// returns the evaluation results. Results may include both triggered and
	// non-triggered entries depending on the implementation.
	Evaluate(ctx context.Context, snapshot PositionSnapshot, rules []Rule) ([]EvalResult, error)
}

// StrategyEvaluator is implemented by individual trading strategies (e.g.
// trailing-stop, fixed-percentage, ATR-based). The engine dispatches to the
// correct StrategyEvaluator based on Rule.StrategyID.
type StrategyEvaluator interface {
	// StrategyID returns the unique identifier that maps this evaluator to
	// rules that reference the same strategy.
	StrategyID() string

	// Evaluate checks a single rule against the current position snapshot and
	// returns the evaluation outcome.
	Evaluate(ctx context.Context, snapshot PositionSnapshot, rule Rule) (EvalResult, error)
}
