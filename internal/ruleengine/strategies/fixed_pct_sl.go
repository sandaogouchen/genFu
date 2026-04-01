package strategies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sandaogouchen/genFu/internal/ruleengine"
)

type FixedPctSL struct{}

func NewFixedPctSL() *FixedPctSL { return &FixedPctSL{} }

func (s *FixedPctSL) StrategyID() string { return "fixed_pct_sl" }

func (s *FixedPctSL) Evaluate(ctx context.Context, snap ruleengine.PositionSnapshot, rule ruleengine.Rule) (ruleengine.EvalResult, error) {
	var params ruleengine.FixedPctSLParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return ruleengine.EvalResult{}, fmt.Errorf("unmarshal FixedPctSLParams: %w", err)
	}

	stopLine := snap.AvgCost * (1 - params.ThresholdPct)
	triggered := snap.MarketPrice <= stopLine && snap.MarketPrice > 0

	return ruleengine.EvalResult{
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		RuleType:     rule.Type,
		Symbol:       snap.Symbol,
		Triggered:    triggered,
		TriggerPrice: stopLine,
		MarketPrice:  snap.MarketPrice,
		PnLPct:       snap.PnLPct,
		Reason:       fmt.Sprintf("固定百分比止损: 当前价 %.2f %s 止损线 %.2f (成本 %.2f × %.0f%%)", snap.MarketPrice, cmp(triggered), stopLine, snap.AvgCost, (1-params.ThresholdPct)*100),
		Action:       rule.Action,
		Priority:     rule.Priority,
		EvalTime:     time.Now(),
	}, nil
}

func cmp(t bool) string {
	if t {
		return "≤"
	}
	return ">"
}
