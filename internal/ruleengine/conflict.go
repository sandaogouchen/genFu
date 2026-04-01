package ruleengine

import "sort"

// ConflictResolver determines which triggered results should be kept when
// multiple rules fire simultaneously for the same position.
type ConflictResolver interface {
	Resolve(results []EvalResult) []EvalResult
}

// DefaultConflictResolver implements the PRD conflict resolution logic:
//
//  1. Only triggered results are considered.
//  2. stop_loss results: keep only the highest-priority one (lowest Priority
//     number). If priorities tie, keep the one with the highest TriggerPrice.
//  3. take_profit results: keep all (partial TP), sorted by priority (ascending).
//  4. If both stop_loss and take_profit fired, stop_loss results come first.
type DefaultConflictResolver struct{}

// NewDefaultConflictResolver returns a ready-to-use DefaultConflictResolver.
func NewDefaultConflictResolver() *DefaultConflictResolver {
	return &DefaultConflictResolver{}
}

// Resolve applies the conflict resolution rules and returns the resolved set.
func (r *DefaultConflictResolver) Resolve(results []EvalResult) []EvalResult {
	if len(results) == 0 {
		return nil
	}

	// Partition into triggered stop-loss and take-profit buckets.
	var stopLoss, takeProfit []EvalResult
	for _, res := range results {
		if !res.Triggered {
			continue
		}
		switch res.RuleType {
		case "stop_loss":
			stopLoss = append(stopLoss, res)
		case "take_profit":
			takeProfit = append(takeProfit, res)
		default:
			// Unknown type -- treat as take-profit (keep all).
			takeProfit = append(takeProfit, res)
		}
	}

	// Resolve stop-loss: pick single highest-priority entry.
	var resolvedSL []EvalResult
	if len(stopLoss) > 0 {
		best := stopLoss[0]
		for _, sl := range stopLoss[1:] {
			if sl.Priority < best.Priority {
				best = sl
			} else if sl.Priority == best.Priority && sl.TriggerPrice > best.TriggerPrice {
				best = sl
			}
		}
		resolvedSL = []EvalResult{best}
	}

	// Resolve take-profit: keep all, sorted ascending by priority.
	sort.Slice(takeProfit, func(i, j int) bool {
		if takeProfit[i].Priority != takeProfit[j].Priority {
			return takeProfit[i].Priority < takeProfit[j].Priority
		}
		return takeProfit[i].TriggerPrice < takeProfit[j].TriggerPrice
	})

	// Merge: stop-loss first, then take-profit.
	out := make([]EvalResult, 0, len(resolvedSL)+len(takeProfit))
	out = append(out, resolvedSL...)
	out = append(out, takeProfit...)
	return out
}
