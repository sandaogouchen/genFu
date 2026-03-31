package news

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

var exposureRelationWeights = map[ExposureRelation]float64{
	ExposureRelationDirect:     1.0,
	ExposureRelationProduct:    0.9,
	ExposureRelationCompetitor: 0.75,
	ExposureRelationSupply:     0.7,
	ExposureRelationTheme:      0.6,
	ExposureRelationMacro:      0.65,
	ExposureRelationUnknown:    0.5,
}

var verificationPenalty = map[VerificationVerdict]float64{
	VerificationVerdictPassed:  1.0,
	VerificationVerdictWeak:    0.7,
	VerificationVerdictInvalid: 0.4,
}

// BuildExposureMapping maps impact items to portfolio holdings + watchlist.
func BuildExposureMapping(event *NewsEvent, mapping ImpactMapping, portfolio *PortfolioContext) []PortfolioExposure {
	if portfolio == nil || len(mapping.Items) == 0 {
		return nil
	}

	agg := make(map[string]PortfolioExposure)
	for _, item := range mapping.Items {
		if strings.TrimSpace(item.EntityName) == "" {
			continue
		}
		for _, h := range portfolio.Holdings {
			relation := resolveHoldingRelation(item, h, portfolio)
			if relation == ExposureRelationUnknown {
				continue
			}
			exp := newExposureFromHolding(item, h, relation, mapping.EventSummary)
			mergeExposure(agg, exposureKey(exp), exp)
		}
		for _, w := range portfolio.Watchlist {
			relation := resolveWatchlistRelation(item, w, portfolio)
			if relation == ExposureRelationUnknown {
				continue
			}
			exp := newExposureFromWatchlist(item, w, relation, mapping.EventSummary)
			mergeExposure(agg, exposureKey(exp), exp)
		}
	}

	if len(agg) == 0 {
		return nil
	}

	out := make([]PortfolioExposure, 0, len(agg))
	for _, exp := range agg {
		out = append(out, exp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ExposureScore > out[j].ExposureScore
	})
	return out
}

func resolveHoldingRelation(item ImpactItem, holding Holding, portfolio *PortfolioContext) ExposureRelation {
	if matchesEntity(item, holding.Name, holding.Code) {
		return ExposureRelationDirect
	}
	for _, product := range holding.Products {
		if matchesEntity(item, product) {
			return ExposureRelationProduct
		}
	}
	for _, competitor := range holding.Competitors {
		if matchesEntity(item, competitor) {
			return ExposureRelationCompetitor
		}
	}
	for _, supplier := range holding.SupplyChain {
		if matchesEntity(item, supplier) {
			return ExposureRelationSupply
		}
	}
	for _, theme := range portfolio.IndustryThemes {
		if matchesEntity(item, theme) {
			return ExposureRelationTheme
		}
	}
	for _, factor := range portfolio.MacroFactors {
		if matchesEntity(item, factor) {
			return ExposureRelationMacro
		}
	}
	return ExposureRelationUnknown
}

func resolveWatchlistRelation(item ImpactItem, watch WatchItem, portfolio *PortfolioContext) ExposureRelation {
	if matchesEntity(item, watch.Name, watch.Code) {
		return ExposureRelationDirect
	}
	for _, theme := range portfolio.IndustryThemes {
		if matchesEntity(item, theme) {
			return ExposureRelationTheme
		}
	}
	for _, factor := range portfolio.MacroFactors {
		if matchesEntity(item, factor) {
			return ExposureRelationMacro
		}
	}
	return ExposureRelationUnknown
}

func matchesEntity(item ImpactItem, candidates ...string) bool {
	entityName := strings.ToLower(strings.TrimSpace(item.EntityName))
	entityCode := strings.ToLower(strings.TrimSpace(item.EntityCode))
	if entityName == "" && entityCode == "" {
		return false
	}
	for _, candidate := range candidates {
		c := strings.ToLower(strings.TrimSpace(candidate))
		if c == "" {
			continue
		}
		if entityName != "" && (strings.Contains(entityName, c) || strings.Contains(c, entityName)) {
			return true
		}
		if entityCode != "" && (strings.Contains(entityCode, c) || strings.Contains(c, entityCode)) {
			return true
		}
	}
	return false
}

func newExposureFromHolding(item ImpactItem, holding Holding, relation ExposureRelation, fallbackReason string) PortfolioExposure {
	itemConfidence := clamp01(maxFloat(item.Confidence, 0.4))
	relationWeight := exposureRelationWeights[relation]
	if relationWeight <= 0 {
		relationWeight = 0.5
	}
	positionFactor := 0.5 + 0.5*clamp01(holding.Weight)
	exposureScore := clamp01(math.Abs(item.ImpactScore) * itemConfidence * relationWeight * positionFactor)
	confidence := clamp01(itemConfidence * relationWeight)
	reason := strings.TrimSpace(item.Rationale)
	if reason == "" {
		reason = fallbackReason
	}
	return PortfolioExposure{
		AssetName:      holding.Name,
		AssetCode:      holding.Code,
		Bucket:         ExposureBucketHolding,
		Relation:       relation,
		Direction:      item.Direction,
		ImpactScore:    clampImpactScore(item.ImpactScore),
		ExposureScore:  exposureScore,
		PositionWeight: clamp01(holding.Weight),
		Confidence:     confidence,
		Rationale:      reason,
	}
}

func newExposureFromWatchlist(item ImpactItem, watch WatchItem, relation ExposureRelation, fallbackReason string) PortfolioExposure {
	itemConfidence := clamp01(maxFloat(item.Confidence, 0.4))
	relationWeight := exposureRelationWeights[relation]
	if relationWeight <= 0 {
		relationWeight = 0.5
	}
	bucketFactor := 0.8
	exposureScore := clamp01(math.Abs(item.ImpactScore) * itemConfidence * relationWeight * bucketFactor)
	confidence := clamp01(itemConfidence * relationWeight)
	reason := strings.TrimSpace(item.Rationale)
	if reason == "" {
		reason = fallbackReason
	}
	return PortfolioExposure{
		AssetName:      watch.Name,
		AssetCode:      watch.Code,
		Bucket:         ExposureBucketWatchlist,
		Relation:       relation,
		Direction:      item.Direction,
		ImpactScore:    clampImpactScore(item.ImpactScore),
		ExposureScore:  exposureScore,
		PositionWeight: 0,
		Confidence:     confidence,
		Rationale:      reason,
	}
}

func exposureKey(exp PortfolioExposure) string {
	asset := strings.TrimSpace(exp.AssetCode)
	if asset == "" {
		asset = strings.TrimSpace(exp.AssetName)
	}
	return fmt.Sprintf("%s|%s", exp.Bucket, strings.ToLower(asset))
}

func mergeExposure(store map[string]PortfolioExposure, key string, next PortfolioExposure) {
	existing, ok := store[key]
	if !ok {
		store[key] = next
		return
	}
	if existing.Direction != next.Direction {
		existing.Direction = DirectionMixed
	}
	if next.ExposureScore > existing.ExposureScore {
		existing.ExposureScore = next.ExposureScore
		existing.Relation = next.Relation
		existing.ImpactScore = next.ImpactScore
	}
	if next.Confidence > existing.Confidence {
		existing.Confidence = next.Confidence
	}
	if strings.TrimSpace(existing.Rationale) == "" {
		existing.Rationale = next.Rationale
	}
	if next.PositionWeight > existing.PositionWeight {
		existing.PositionWeight = next.PositionWeight
	}
	store[key] = existing
}

func mergeEventEntities(event *NewsEvent, mapping ImpactMapping) []EntityLabel {
	entityMap := make(map[string]EntityLabel)
	if event != nil {
		for _, entity := range event.Labels.Entities {
			key := strings.ToLower(strings.TrimSpace(entity.Name))
			if key == "" {
				continue
			}
			entityMap[key] = entity
		}
	}
	for _, item := range mapping.Items {
		name := strings.TrimSpace(item.EntityName)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := entityMap[key]; !exists {
			entityMap[key] = EntityLabel{
				Name:      name,
				Code:      item.EntityCode,
				Role:      EntityRoleMentioned,
				Relevance: clamp01(maxFloat(item.Confidence, 0.5)),
			}
		}
	}
	entities := make([]EntityLabel, 0, len(entityMap))
	for _, entity := range entityMap {
		if entity.Role == "" {
			entity.Role = EntityRoleMentioned
		}
		if entity.Relevance <= 0 {
			entity.Relevance = 0.5
		}
		entities = append(entities, entity)
	}
	sort.Slice(entities, func(i, j int) bool {
		return entities[i].Relevance > entities[j].Relevance
	})
	if len(entities) > 0 && entities[0].Role == EntityRoleMentioned {
		entities[0].Role = EntityRolePrimary
	}
	return entities
}

func backfillLegacyFromRiskMapping(event *NewsEvent, mapping ImpactMapping, exposures []PortfolioExposure) {
	if event == nil {
		return
	}
	if event.FunnelResult == nil {
		event.FunnelResult = &FunnelResult{}
	}
	fr := event.FunnelResult

	fr.EventEntities = mergeEventEntities(event, mapping)
	fr.ImpactMapping = &mapping
	if len(exposures) > 0 {
		fr.ExposureMapping = exposures
	}
	if len(mapping.MonitoringSignals) > 0 {
		fr.MonitoringSignalsV2 = mapping.MonitoringSignals
	}
	if fr.L2CausalSketch == "" {
		fr.L2CausalSketch = mapping.EventSummary
	}

	if len(fr.ExposureMapping) == 0 {
		backfillLegacyMonitoringSignals(event)
		return
	}

	affectedAssets, maxScore, hasHolding, hasConflict := deriveLegacyAssets(fr.ExposureMapping)
	if len(affectedAssets) > 0 {
		fr.L2AffectedAssets = affectedAssets
	}

	derivedRelevance := RelevanceLow
	switch {
	case hasHolding && maxScore >= 0.4:
		derivedRelevance = RelevanceHigh
	case hasHolding || maxScore >= 0.2:
		derivedRelevance = RelevanceMedium
	}
	if compareRelevance(derivedRelevance, fr.L2Relevance) > 0 {
		fr.L2Relevance = derivedRelevance
	}

	derivedPriority := derivePriority(maxScore)
	if derivedPriority > fr.L2Priority {
		fr.L2Priority = derivedPriority
	}
	if fr.L2Priority == 0 {
		fr.L2Priority = derivedPriority
	}
	if fr.L2Priority >= 4 || hasConflict {
		fr.L2NeedsDeep = true
	}
	if fr.L2Relevance == RelevanceHigh || fr.L2Relevance == RelevanceMedium {
		fr.L2Pass = true
	}

	backfillLegacyMonitoringSignals(event)
}

func deriveLegacyAssets(exposures []PortfolioExposure) ([]AssetImpact, float64, bool, bool) {
	type agg struct {
		direction Direction
		conf      float64
		sketch    string
		isHolding bool
	}
	store := map[string]*agg{}
	meta := map[string]PortfolioExposure{}
	maxScore := 0.0
	hasHolding := false
	hasConflict := false
	for _, exp := range exposures {
		key := strings.TrimSpace(exp.AssetCode)
		if key == "" {
			key = strings.TrimSpace(exp.AssetName)
		}
		if key == "" {
			continue
		}
		if exp.Bucket == ExposureBucketHolding {
			hasHolding = true
		}
		score := math.Abs(exp.ExposureScore)
		if score > maxScore {
			maxScore = score
		}
		entry, ok := store[key]
		if !ok {
			store[key] = &agg{
				direction: exp.Direction,
				conf:      clamp01(maxFloat(exp.Confidence, 0.5)),
				sketch:    exp.Rationale,
				isHolding: exp.Bucket == ExposureBucketHolding,
			}
			meta[key] = exp
			continue
		}
		if entry.direction != exp.Direction {
			entry.direction = DirectionMixed
			hasConflict = true
		}
		if exp.Confidence > entry.conf {
			entry.conf = exp.Confidence
		}
		entry.isHolding = entry.isHolding || exp.Bucket == ExposureBucketHolding
		if strings.TrimSpace(entry.sketch) == "" {
			entry.sketch = exp.Rationale
		}
	}
	assets := make([]AssetImpact, 0, len(store))
	for key, val := range store {
		item := meta[key]
		assets = append(assets, AssetImpact{
			AssetName:    item.AssetName,
			AssetCode:    item.AssetCode,
			Direction:    val.direction,
			Confidence:   clamp01(val.conf),
			IsHolding:    val.isHolding,
			CausalSketch: val.sketch,
		})
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Confidence > assets[j].Confidence
	})
	return assets, maxScore, hasHolding, hasConflict
}

func derivePriority(maxScore float64) int {
	switch {
	case maxScore >= 0.8:
		return 5
	case maxScore >= 0.6:
		return 4
	case maxScore >= 0.4:
		return 3
	case maxScore >= 0.2:
		return 2
	default:
		return 1
	}
}

func compareRelevance(a, b Relevance) int {
	score := func(r Relevance) int {
		switch r {
		case RelevanceHigh:
			return 4
		case RelevanceMedium:
			return 3
		case RelevanceLow:
			return 2
		case RelevanceNone:
			return 1
		default:
			return 0
		}
	}
	return score(a) - score(b)
}

func applyVerificationPenalty(event *NewsEvent, verification CausalVerification) {
	if event == nil || event.FunnelResult == nil {
		return
	}
	fr := event.FunnelResult
	fr.CausalVerification = &verification
	penalty, ok := verificationPenalty[verification.Verdict]
	if !ok {
		penalty = 1.0
	}
	if penalty >= 1 {
		backfillLegacyMonitoringSignals(event)
		return
	}

	if fr.L2Priority > 0 {
		fr.L2Priority = maxInt(1, int(math.Round(float64(fr.L2Priority)*penalty)))
	}
	for i := range fr.L2AffectedAssets {
		base := fr.L2AffectedAssets[i].Confidence
		if base <= 0 {
			base = 0.5
		}
		fr.L2AffectedAssets[i].Confidence = clamp01(base * penalty)
	}
	for i := range fr.ExposureMapping {
		fr.ExposureMapping[i].Confidence = clamp01(fr.ExposureMapping[i].Confidence * penalty)
		fr.ExposureMapping[i].ExposureScore = clamp01(fr.ExposureMapping[i].ExposureScore * penalty)
	}
	if strings.TrimSpace(fr.L2CausalSketch) == "" && strings.TrimSpace(verification.Reason) != "" {
		fr.L2CausalSketch = verification.Reason
	}
	if len(fr.MonitoringSignalsV2) == 0 && verification.Verdict != VerificationVerdictPassed {
		fr.MonitoringSignalsV2 = append(fr.MonitoringSignalsV2, StructuredMonitoringSignal{
			Signal:    "因果链置信度下降",
			Metric:    "causal_verification_score",
			Operator:  SignalOperatorLessThan,
			Threshold: fmt.Sprintf("%.2f", verification.Score),
			Window:    "1d",
			Reason:    verification.Reason,
		})
	}
	backfillLegacyMonitoringSignals(event)
}

func backfillLegacyMonitoringSignals(event *NewsEvent) {
	if event == nil || event.FunnelResult == nil || len(event.FunnelResult.MonitoringSignalsV2) == 0 {
		return
	}
	if event.FunnelResult.L3Analysis == nil {
		event.FunnelResult.L3Analysis = &CausalAnalysis{}
	}
	existing := map[string]struct{}{}
	for _, signal := range event.FunnelResult.L3Analysis.MonitoringSignals {
		existing[strings.TrimSpace(signal)] = struct{}{}
	}
	for _, structured := range event.FunnelResult.MonitoringSignalsV2 {
		text := formatStructuredSignal(structured)
		if text == "" {
			continue
		}
		if _, ok := existing[text]; ok {
			continue
		}
		event.FunnelResult.L3Analysis.MonitoringSignals = append(event.FunnelResult.L3Analysis.MonitoringSignals, text)
		existing[text] = struct{}{}
	}
}

func formatStructuredSignal(signal StructuredMonitoringSignal) string {
	if strings.TrimSpace(signal.Signal) != "" {
		return strings.TrimSpace(signal.Signal)
	}
	parts := make([]string, 0, 4)
	if signal.Metric != "" {
		parts = append(parts, signal.Metric)
	}
	if signal.Operator != "" {
		parts = append(parts, string(signal.Operator))
	}
	if signal.Threshold != "" {
		parts = append(parts, signal.Threshold)
	}
	if signal.Window != "" {
		parts = append(parts, signal.Window)
	}
	if len(parts) == 0 {
		return ""
	}
	text := strings.Join(parts, " ")
	if strings.TrimSpace(signal.Reason) != "" {
		text += " | " + strings.TrimSpace(signal.Reason)
	}
	return text
}

func convertLegacyMonitoringSignals(event *NewsEvent, legacy []string) []StructuredMonitoringSignal {
	if len(legacy) == 0 {
		return nil
	}
	assets := []string{}
	if event != nil && event.FunnelResult != nil {
		for _, asset := range event.FunnelResult.L2AffectedAssets {
			if strings.TrimSpace(asset.AssetName) != "" {
				assets = append(assets, asset.AssetName)
			}
		}
	}
	out := make([]StructuredMonitoringSignal, 0, len(legacy))
	for _, signal := range legacy {
		text := strings.TrimSpace(signal)
		if text == "" {
			continue
		}
		out = append(out, StructuredMonitoringSignal{
			Signal:    text,
			Metric:    "legacy_signal",
			Operator:  SignalOperatorEqual,
			Threshold: "trigger",
			Window:    "intraday",
			Assets:    assets,
			Reason:    text,
		})
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
