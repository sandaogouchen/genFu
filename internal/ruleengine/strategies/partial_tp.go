package strategies

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sandaogouchen/genFu/internal/ruleengine"
)

type PartialTP struct{}

func NewPartialTP() *PartialTP { return &PartialTP{} }

func (s *PartialTP) StrategyID() string { return "partial_tp" }

func (s *PartialTP) Evaluate(ctx context.Context, snap ruleengine.PositionSnapshot, rule ruleengine.Rule) (ruleengine.EvalResult, error) {
	var params ruleengine.PartialTPParams
	if err := json.Unmarshal(rule.Params, &params); err != nil {
		return ruleengine.EvalResult{}, fmt.Errorf("unmarshal PartialTPParams: %w", err)
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

	for i, tier := range params.Tiers {
		if tier.Executed {
			continue
		}

		targetPrice := snap.AvgCost * (1 + tier.TriggerPct)

		if tier.Trailing {
			// Trailing mode: must first reach trigger, then pull back
			if snap.PnLPct < tier.TriggerPct {
				continue
			}
			trailLine := snap.HighestPrice * (1 - tier.TrailPct)
			if snap.MarketPrice <= trailLine {
				result.Triggered = true
				result.TriggerPrice = trailLine
				result.Action = ruleengine.RuleAction{
					ActionType:  "sell_pct",
					SellPercent: tier.SellPercent,
					Urgency:     rule.Action.Urgency,
				}
				result.Reason = fmt.Sprintf("分批止盈第%d档(追踪): 盈利 %.2f%%, 回撤至 %.2f, 卖出 %.0f%%", i+1, snap.PnLPct*100, trailLine, tier.SellPercent*100)
				return result, nil
			}
		} else {
			// Threshold mode
			if snap.MarketPrice >= targetPrice {
				result.Triggered = true
				result.TriggerPrice = targetPrice
				result.Action = ruleengine.RuleAction{
					ActionType:  "sell_pct",
					SellPercent: tier.SellPercent,
					Urgency:     rule.Action.Urgency,
				}
				result.Reason = fmt.Sprintf("分批止盈第%d档: 当前价 %.2f ≥ 目标 %.2f, 卖出 %.0f%%", i+1, snap.MarketPrice, targetPrice, tier.SellPercent*100)
				return result, nil
			}
		}
	}

	result.Reason = "分批止盈: 无档位触发"
	return result, nil
}
