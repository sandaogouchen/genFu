package strategies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sandaogouchen/genFu/internal/ruleengine"
)

type TrailingSL struct{}

func NewTrailingSL() *TrailingSL { return &TrailingSL{} }

func (s *TrailingSL) StrategyID() string { return "trailing_sl" }

func (s *TrailingSL) Evaluate(ctx context.Context, snap ruleengine.PositionSnapshot, rule ruleengine.Rule) (ruleengine.EvalResult, error) {
	var params ruleengine.TrailingSLParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return ruleengine.EvalResult{}, fmt.Errorf("unmarshal TrailingSLParams: %w", err)
	}

	result := ruleengine.EvalResult{
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		RuleType:    rule.Type,
		Symbol:      snap.Symbol,
		MarketPrice: snap.MarketPrice,
		PnLPct:      snap.PnLPct,
		Action:      rule.Action,
		Priority:    rule.Priority,
		EvalTime:    time.Now(),
	}

	// Check activation gate
	if params.ActivationPct > 0 && snap.PnLPct < params.ActivationPct {
		result.Reason = fmt.Sprintf("追踪止损未激活: 盈利 %.2f%% < 激活门槛 %.2f%%", snap.PnLPct*100, params.ActivationPct*100)
		return result, nil
	}

	stopLine := snap.HighestPrice * (1 - params.TrailPct)
	result.TriggerPrice = stopLine
	result.Triggered = snap.MarketPrice <= stopLine && snap.MarketPrice > 0
	result.Reason = fmt.Sprintf("追踪止损: 当前价 %.2f, 最高价 %.2f, 止损线 %.2f (回撤 %.0f%%)", snap.MarketPrice, snap.HighestPrice, stopLine, params.TrailPct*100)

	return result, nil
}
