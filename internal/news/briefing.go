// Package news implements briefing generator.
// Generates 6-module investment briefing from filtered events.
package news

import (
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// Briefing Generator
// ──────────────────────────────────────────────

// Generator represents briefing generator
type Generator struct {
	portfolio *PortfolioContext
}

// NewGenerator creates a new briefing generator
func NewGenerator(portfolio *PortfolioContext) *Generator {
	return &Generator{portfolio: portfolio}
}

// Generate generates a complete briefing from filtered events
func (g *Generator) Generate(events []*NewsEvent, triggerType TriggerType, totalCollected, l1Passed, l2Passed int) *Briefing {
	briefID := fmt.Sprintf("brief_%d", time.Now().Unix())

	brief := &Briefing{
		ID:                 briefID,
		GeneratedAt:        time.Now(),
		TriggerType:        triggerType,
		Period:             g.getPeriodName(triggerType),
		TotalNewsProcessed: totalCollected,
		L1Passed:           l1Passed,
		L2Passed:           l2Passed,
		L3Analyzed:         g.countL3Analyzed(events),
	}

	// Module 1: Macro Overview
	brief.MacroOverview = g.generateMacroOverview(events)

	// Module 2: Portfolio Impact Matrix
	brief.PortfolioImpact = g.generatePortfolioImpact(events)

	// Module 3: Opportunities
	brief.Opportunities = g.generateOpportunities(events)

	// Module 4: Risk Alerts
	brief.RiskAlerts = g.generateRiskAlerts(events)

	// Module 5: Conflict Signals
	brief.ConflictSignals = g.generateConflictSignals(events)

	// Module 6: Monitoring Items
	brief.MonitoringItems = g.generateMonitoringItems(events)

	return brief
}

func (g *Generator) getPeriodName(triggerType TriggerType) string {
	now := time.Now()
	switch triggerType {
	case TriggerPreMarket:
		return fmt.Sprintf("开盘前 %s", now.Format("2006-01-02"))
	case TriggerIntraday:
		return fmt.Sprintf("盘中 %s %s", now.Format("2006-01-02"), now.Format("15:04"))
	case TriggerBreaking:
		return fmt.Sprintf("突发 %s", now.Format("2006-01-02 15:04"))
	case TriggerManual:
		return fmt.Sprintf("手动 %s", now.Format("2006-01-02 15:04"))
	default:
		return now.Format("2006-01-02")
	}
}

func (g *Generator) countL3Analyzed(events []*NewsEvent) int {
	count := 0
	for _, e := range events {
		if e.FunnelResult != nil && e.FunnelResult.L3Analysis != nil {
			count++
		}
	}
	return count
}

// ──────────────────────────────────────────────
// Module 1: Macro Overview
// ──────────────────────────────────────────────

func (g *Generator) generateMacroOverview(events []*NewsEvent) MacroOverview {
	overview := MacroOverview{
		Summary:         "市场整体平稳",
		MarketSentiment: SentimentNeutral,
		KeyFactors:      []MacroFactor{},
		RiskLevel:       "medium",
	}

	// Collect macro events
	var macroEvents []*NewsEvent
	for _, e := range events {
		for _, d := range e.Domains {
			if d == DomainMacro {
				macroEvents = append(macroEvents, e)
				break
			}
		}
	}

	if len(macroEvents) == 0 {
		return overview
	}

	// Calculate average sentiment
	totalSentiment := 0.0
	for _, e := range macroEvents {
		totalSentiment += e.Labels.Sentiment
	}
	avgSentiment := totalSentiment / float64(len(macroEvents))
	overview.MarketSentiment = ToSentimentLevel(avgSentiment)

	// Determine risk level
	if avgSentiment < -0.4 {
		overview.RiskLevel = "high"
	} else if avgSentiment < -0.1 {
		overview.RiskLevel = "medium"
	} else {
		overview.RiskLevel = "low"
	}

	// Generate key factors
	for _, e := range macroEvents {
		if len(overview.KeyFactors) >= 5 {
			break
		}
		factor := MacroFactor{
			Factor:    e.Title,
			Direction: getDirectionFromSentiment(e.Labels.Sentiment),
			Impact:    getTimeframeImpact(e.Labels.Timeframe),
		}
		overview.KeyFactors = append(overview.KeyFactors, factor)
	}

	// Generate summary
	overview.Summary = g.generateMacroSummary(macroEvents, overview)

	return overview
}

func (g *Generator) generateMacroSummary(events []*NewsEvent, overview MacroOverview) string {
	if len(events) == 0 {
		return "暂无重大宏观事件"
	}

	sentimentText := "中性"
	switch overview.MarketSentiment {
	case SentimentVeryPositive, SentimentPositive:
		sentimentText = "偏乐观"
	case SentimentVeryNegative, SentimentNegative:
		sentimentText = "偏谨慎"
	}

	eventCount := len(events)
	return fmt.Sprintf("共监测到 %d 条宏观事件，市场情绪%s，风险水平%s", eventCount, sentimentText, overview.RiskLevel)
}

// ──────────────────────────────────────────────
// Module 2: Portfolio Impact Matrix
// ──────────────────────────────────────────────

func (g *Generator) generatePortfolioImpact(events []*NewsEvent) []PortfolioImpactRow {
	if g.portfolio == nil {
		return nil
	}

	impactMap := make(map[string]*PortfolioImpactRow)

	// Initialize with holdings
	for _, h := range g.portfolio.Holdings {
		impactMap[h.Code] = &PortfolioImpactRow{
			Asset:         h.Name,
			Code:          h.Code,
			RelatedEvents: []string{},
			NetDirection:  DirectionUncertain,
			Confidence:    0.0,
			Urgency:       "monitor",
			Action:        "持续观察",
			KeyCausal:     "",
		}
	}

	// Process events
	for _, e := range events {
		if e.FunnelResult == nil {
			continue
		}

		// Prefer exposure mapping from risk-input upgrade.
		if len(e.FunnelResult.ExposureMapping) > 0 {
			for _, exposure := range e.FunnelResult.ExposureMapping {
				key := exposure.AssetCode
				if key == "" {
					key = exposure.AssetName
				}
				if key == "" {
					continue
				}

				row, ok := impactMap[key]
				if !ok {
					action := "观察"
					if exposure.Bucket == ExposureBucketHolding {
						action = "持续观察"
					} else if exposure.Bucket == ExposureBucketWatchlist {
						action = "观察池跟踪"
					}
					row = &PortfolioImpactRow{
						Asset:         exposure.AssetName,
						Code:          exposure.AssetCode,
						RelatedEvents: []string{},
						NetDirection:  DirectionUncertain,
						Confidence:    0.0,
						Urgency:       "monitor",
						Action:        action,
						KeyCausal:     "",
					}
					impactMap[key] = row
				}

				row.RelatedEvents = append(row.RelatedEvents, e.Title)
				if row.NetDirection == DirectionUncertain {
					row.NetDirection = exposure.Direction
				} else if row.NetDirection != exposure.Direction {
					row.NetDirection = DirectionMixed
				}
				exposureConfidence := exposure.Confidence
				if exposure.ExposureScore > exposureConfidence {
					exposureConfidence = exposure.ExposureScore
				}
				if exposureConfidence > row.Confidence {
					row.Confidence = exposureConfidence
				}
				if e.FunnelResult.L2Priority >= 4 {
					row.Urgency = "immediate"
					row.Action = "立即评估"
				} else if e.FunnelResult.L2Priority >= 3 && row.Urgency != "immediate" {
					row.Urgency = "today"
					row.Action = "今日关注"
				}
				if row.KeyCausal == "" {
					if exposure.Rationale != "" {
						row.KeyCausal = exposure.Rationale
					} else if e.FunnelResult.L2CausalSketch != "" {
						row.KeyCausal = e.FunnelResult.L2CausalSketch
					}
				}
			}
			continue
		}

		// Backward-compatible fallback to legacy L2 affected assets.
		for _, impact := range e.FunnelResult.L2AffectedAssets {
			key := impact.AssetCode
			if key == "" {
				key = impact.AssetName
			}
			row, ok := impactMap[key]
			if !ok {
				row = &PortfolioImpactRow{
					Asset:         impact.AssetName,
					Code:          impact.AssetCode,
					RelatedEvents: []string{},
					NetDirection:  DirectionUncertain,
					Confidence:    0.0,
					Urgency:       "monitor",
					Action:        "观察",
					KeyCausal:     "",
				}
				impactMap[key] = row
			}

			row.RelatedEvents = append(row.RelatedEvents, e.Title)
			if row.NetDirection == DirectionUncertain {
				row.NetDirection = impact.Direction
			} else if row.NetDirection != impact.Direction {
				row.NetDirection = DirectionMixed
			}
			if impact.Confidence > row.Confidence {
				row.Confidence = impact.Confidence
			}
			if e.FunnelResult.L2Priority >= 4 {
				row.Urgency = "immediate"
				row.Action = "立即评估"
			} else if e.FunnelResult.L2Priority >= 3 && row.Urgency != "immediate" {
				row.Urgency = "today"
				row.Action = "今日关注"
			}
			if e.FunnelResult.L2CausalSketch != "" && row.KeyCausal == "" {
				row.KeyCausal = e.FunnelResult.L2CausalSketch
			}
		}
	}

	// Convert to slice
	result := make([]PortfolioImpactRow, 0)
	for _, row := range impactMap {
		result = append(result, *row)
	}

	return result
}

// ──────────────────────────────────────────────
// Module 3: Opportunities
// ──────────────────────────────────────────────

func (g *Generator) generateOpportunities(events []*NewsEvent) []OpportunityAlert {
	var opportunities []OpportunityAlert

	for _, e := range events {
		if e.FunnelResult == nil {
			continue
		}

		// From L3 cross-asset impacts
		if e.FunnelResult.L3Analysis != nil {
			for _, impact := range e.FunnelResult.L3Analysis.CrossAssetImpacts {
				if !impact.IsHolding && impact.Confidence >= 0.6 {
					opportunities = append(opportunities, OpportunityAlert{
						Source:       "L3",
						Asset:        impact.Asset,
						AssetCode:    impact.AssetCode,
						Direction:    impact.Direction,
						Mechanism:    impact.Mechanism,
						Confidence:   impact.Confidence,
						SourceEvents: []string{e.Title},
					})
				}
			}
		}

		// From L2 non-holding assets
		for _, impact := range e.FunnelResult.L2AffectedAssets {
			if !impact.IsHolding && e.FunnelResult.L2Relevance == RelevanceHigh {
				opportunities = append(opportunities, OpportunityAlert{
					Source:       "L2",
					Asset:        impact.AssetName,
					AssetCode:    impact.AssetCode,
					Direction:    impact.Direction,
					Mechanism:    impact.CausalSketch,
					Confidence:   0.5, // Default confidence for L2
					SourceEvents: []string{e.Title},
				})
			}
		}
	}

	// Limit to top 10
	if len(opportunities) > 10 {
		opportunities = opportunities[:10]
	}

	return opportunities
}

// ──────────────────────────────────────────────
// Module 4: Risk Alerts
// ──────────────────────────────────────────────

func (g *Generator) generateRiskAlerts(events []*NewsEvent) []RiskAlert {
	var alerts []RiskAlert

	for _, e := range events {
		if e.FunnelResult == nil {
			continue
		}

		// High priority bearish events
		if e.FunnelResult.L2Priority >= 4 {
			for _, impact := range e.FunnelResult.L2AffectedAssets {
				if impact.Direction == DirectionBearish && impact.IsHolding {
					level := "warning"
					if e.FunnelResult.L2Priority >= 5 {
						level = "critical"
					}

					alerts = append(alerts, RiskAlert{
						Level:       level,
						Description: e.Title,
						Assets:      []string{impact.AssetName},
						Events:      []string{e.Title},
						Action:      g.getRiskAction(level),
					})
				}
			}
		}
	}

	// Limit to top 5
	if len(alerts) > 5 {
		alerts = alerts[:5]
	}

	return alerts
}

func (g *Generator) getRiskAction(level string) string {
	switch level {
	case "critical":
		return "立即评估风险敞口"
	case "warning":
		return "密切监控"
	default:
		return "观察"
	}
}

// ──────────────────────────────────────────────
// Module 5: Conflict Signals
// ──────────────────────────────────────────────

func (g *Generator) generateConflictSignals(events []*NewsEvent) []ConflictSignal {
	// Group events by asset
	assetEvents := make(map[string][]*NewsEvent)
	for _, e := range events {
		if e.FunnelResult == nil {
			continue
		}
		for _, impact := range e.FunnelResult.L2AffectedAssets {
			assetEvents[impact.AssetName] = append(assetEvents[impact.AssetName], e)
		}
	}

	var signals []ConflictSignal

	for asset, events := range assetEvents {
		// Find bullish and bearish events
		var bullishEvent, bearishEvent string
		for _, e := range events {
			if e.FunnelResult == nil {
				continue
			}
			for _, impact := range e.FunnelResult.L2AffectedAssets {
				if impact.AssetName == asset || impact.AssetCode == asset {
					if impact.Direction == DirectionBullish && bullishEvent == "" {
						bullishEvent = e.Title
					} else if impact.Direction == DirectionBearish && bearishEvent == "" {
						bearishEvent = e.Title
					}
				}
			}
		}

		// Create conflict signal if both exist
		if bullishEvent != "" && bearishEvent != "" {
			signals = append(signals, ConflictSignal{
				Asset:        asset,
				BullishEvent: bullishEvent,
				BearishEvent: bearishEvent,
				Analysis:     "存在多空冲突信号，需综合评估",
			})
		}
	}

	// Limit to top 5
	if len(signals) > 5 {
		signals = signals[:5]
	}

	return signals
}

// ──────────────────────────────────────────────
// Module 6: Monitoring Items
// ──────────────────────────────────────────────

func (g *Generator) generateMonitoringItems(events []*NewsEvent) []MonitoringItem {
	var items []MonitoringItem

	for _, e := range events {
		if e.FunnelResult == nil {
			continue
		}

		// Prefer structured monitoring signals.
		if len(e.FunnelResult.MonitoringSignalsV2) > 0 {
			for _, signal := range e.FunnelResult.MonitoringSignalsV2 {
				items = append(items, MonitoringItem{
					Signal:    signal.Signal,
					Threshold: formatStructuredSignal(signal),
					Assets:    signal.Assets,
					Reason:    signal.Reason,
				})
			}
			continue
		}

		// Backward-compatible fallback from L3 analysis.
		if e.FunnelResult.L3Analysis == nil {
			continue
		}
		for _, signal := range e.FunnelResult.L3Analysis.MonitoringSignals {
			// Extract affected assets
			var assets []string
			for _, impact := range e.FunnelResult.L2AffectedAssets {
				assets = append(assets, impact.AssetName)
			}

			items = append(items, MonitoringItem{
				Signal:    signal,
				Threshold: "市场预期变化",
				Assets:    assets,
				Reason:    e.Title,
			})
		}
	}

	// Limit to top 10
	if len(items) > 10 {
		items = items[:10]
	}

	return items
}

// ──────────────────────────────────────────────
// Helper Functions
// ──────────────────────────────────────────────

func getDirectionFromSentiment(sentiment float64) Direction {
	if sentiment > 0.2 {
		return DirectionBullish
	} else if sentiment < -0.2 {
		return DirectionBearish
	}
	return DirectionUncertain
}

func getTimeframeImpact(timeframe Timeframe) string {
	switch timeframe {
	case TimeframeImmediate:
		return "high"
	case TimeframeShort:
		return "medium"
	case TimeframeMedium, TimeframeLong:
		return "low"
	default:
		return "medium"
	}
}
