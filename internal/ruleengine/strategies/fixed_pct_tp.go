package strategies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sandaogouchen/genFu/internal/ruleengine"
)

type FixedPctTP struct{}

func NewFixedPctTP() *FixedPctTP { return &FixedPctTP{} }

func (s *FixedPctTP) StrategyID() string { return "fixed_pct_tp" }

func (s *FixedPctTP) Evaluate(ctx context.Context, snap ruleengine.PositionSnapshot, rule ruleengine.Rule) (ruleengine.EvalResult, error) {
	var params ruleengine.FixedPctTPParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return ruleengine.EvalResult{}, fmt.Errorf("unmarshal FixedPctTPParams: %w", err)
	}

	targetPrice := snap.AvgCost * (1 + params.TargetPct)
	triggered := snap.MarketPrice >= targetPrice

	return ruleengine.EvalResult{
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		RuleType:     rule.Type,
		Symbol:       snap.Symbol,
		Triggered:    triggered,
		TriggerPrice: targetPrice,
		MarketPrice:  snap.MarketPrice,
		PnLPct:       snap.PnLPct,
		Reason:       fmt.Sprintf("固定百分比止盈: 当前价 %.2f, 目标价 %.2f (成本 %.2f + %.0f%%)", snap.MarketPrice, targetPrice, snap.AvgCost, params.TargetPct*100),
		Action:       rule.Action,
		Priority:     rule.Priority,
		EvalTime:     time.Now(),
	}, nil
}
