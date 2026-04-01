package strategies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sandaogouchen/genFu/internal/ruleengine"
)

type ATRSL struct{}

func NewATRSL() *ATRSL { return &ATRSL{} }

func (s *ATRSL) StrategyID() string { return "atr_sl" }

func (s *ATRSL) Evaluate(ctx context.Context, snap ruleengine.PositionSnapshot, rule ruleengine.Rule) (ruleengine.EvalResult, error) {
	var params ruleengine.ATRSLParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return ruleengine.EvalResult{}, fmt.Errorf("unmarshal ATRSLParams: %w", err)
	}

	atrKey := fmt.Sprintf("atr_%d", params.Period)
	atrVal, ok := snap.Indicators[atrKey]
	if !ok {
		return ruleengine.EvalResult{}, fmt.Errorf("indicator %q not available in snapshot", atrKey)
	}

	stopLine := snap.HighestPrice - params.Multiplier*atrVal
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
		Reason:       fmt.Sprintf("ATR止损: 最高价 %.2f - %.1f×ATR(%d)=%.2f, 止损线 %.2f", snap.HighestPrice, params.Multiplier, params.Period, atrVal, stopLine),
		Action:       rule.Action,
		Priority:     rule.Priority,
		EvalTime:     time.Now(),
	}, nil
}
