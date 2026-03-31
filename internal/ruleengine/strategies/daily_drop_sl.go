package strategies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sandaogouchen/genFu/internal/ruleengine"
)

type DailyDropSL struct{}

func NewDailyDropSL() *DailyDropSL { return &DailyDropSL{} }

func (s *DailyDropSL) StrategyID() string { return "daily_drop_sl" }

func (s *DailyDropSL) Evaluate(ctx context.Context, snap ruleengine.PositionSnapshot, rule ruleengine.Rule) (ruleengine.EvalResult, error) {
	var params ruleengine.DailyDropSLParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return ruleengine.EvalResult{}, fmt.Errorf("unmarshal DailyDropSLParams: %w", err)
	}

	triggered := snap.DailyChange <= -params.MaxDailyDropPct

	return ruleengine.EvalResult{
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		RuleType:     rule.Type,
		Symbol:       snap.Symbol,
		Triggered:    triggered,
		TriggerPrice: snap.MarketPrice,
		MarketPrice:  snap.MarketPrice,
		PnLPct:       snap.PnLPct,
		Reason:       fmt.Sprintf("单日跌幅止损: 当日跌幅 %.2f%%, 阈值 %.2f%%", snap.DailyChange*100, params.MaxDailyDropPct*100),
		Action:       rule.Action,
		Priority:     rule.Priority,
		EvalTime:     time.Now(),
	}, nil
}
