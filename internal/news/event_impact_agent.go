package news

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// EventImpactAgent maps events to directional entity impacts.
type EventImpactAgent interface {
	AnalyzeBatch(ctx context.Context, batch []*NewsEvent, portfolio *PortfolioContext) ([]ImpactMapping, error)
}

// LLMEventImpactAgent implements EventImpactAgent with LLM + fallback heuristics.
type LLMEventImpactAgent struct {
	llmSvc    LLMService
	batchSize int
}

func NewLLMEventImpactAgent(llmSvc LLMService, batchSize int) *LLMEventImpactAgent {
	if batchSize <= 0 {
		batchSize = 10
	}
	return &LLMEventImpactAgent{llmSvc: llmSvc, batchSize: batchSize}
}

type eventImpactBatchResponse struct {
	Results []eventImpactResult `json:"results"`
}

type eventImpactResult struct {
	NewsIndex         int                          `json:"news_index"`
	EventSummary      string                       `json:"event_summary"`
	Items             []ImpactItem                 `json:"items"`
	MonitoringSignals []StructuredMonitoringSignal `json:"monitoring_signals"`
}

func (a *LLMEventImpactAgent) AnalyzeBatch(ctx context.Context, batch []*NewsEvent, portfolio *PortfolioContext) ([]ImpactMapping, error) {
	results := make([]ImpactMapping, len(batch))
	for i, event := range batch {
		results[i] = fallbackImpactMapping(event)
	}

	if len(batch) == 0 || a == nil || a.llmSvc == nil {
		return results, nil
	}

	var firstErr error
	for start := 0; start < len(batch); start += a.batchSize {
		end := start + a.batchSize
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[start:end]
		chunkMap, err := a.analyzeChunk(ctx, chunk, portfolio)
		if err != nil && firstErr == nil {
			firstErr = err
		}

		for idx, mapping := range chunkMap {
			globalIdx := start + idx
			if globalIdx >= 0 && globalIdx < len(results) {
				results[globalIdx] = normalizeImpactMapping(mapping, chunk[idx])
			}
		}
	}

	return results, firstErr
}

func (a *LLMEventImpactAgent) analyzeChunk(ctx context.Context, chunk []*NewsEvent, portfolio *PortfolioContext) (map[int]ImpactMapping, error) {
	systemPrompt := `你是事件影响映射引擎。你需要把新闻事件映射为“实体 -> 方向 -> 强度 -> 可执行监控信号”。
约束：
1) impact_score 在 -1.0 到 1.0 之间；
2) impact_level 只能是 weak/moderate/strong；
3) direction 只能是 bullish/bearish/mixed/uncertain；
4) monitoring_signals 需包含可执行字段(metric/operator/threshold/window)；
5) 返回严格 JSON。`

	userPrompt := buildEventImpactUserPrompt(chunk, portfolio)
	resp, err := a.llmSvc.ChatComplete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("event impact llm call failed: %w", err)
	}

	var parsed eventImpactBatchResponse
	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("event impact parse failed: %w", err)
	}

	out := make(map[int]ImpactMapping, len(chunk))
	for _, r := range parsed.Results {
		if r.NewsIndex < 0 || r.NewsIndex >= len(chunk) {
			continue
		}
		out[r.NewsIndex] = ImpactMapping{
			EventSummary:      strings.TrimSpace(r.EventSummary),
			Items:             r.Items,
			MonitoringSignals: r.MonitoringSignals,
		}
	}
	return out, nil
}

func buildEventImpactUserPrompt(chunk []*NewsEvent, portfolio *PortfolioContext) string {
	var sb strings.Builder
	sb.WriteString("## 投资组合上下文\n")
	if portfolio != nil {
		sb.WriteString("持仓:\n")
		for _, h := range portfolio.Holdings {
			sb.WriteString(fmt.Sprintf("- %s(%s) 行业:%s 仓位:%.1f%%\n", h.Name, h.Code, h.Industry, h.Weight*100))
		}
		sb.WriteString("观察池:\n")
		for _, w := range portfolio.Watchlist {
			sb.WriteString(fmt.Sprintf("- %s(%s)\n", w.Name, w.Code))
		}
	}

	sb.WriteString("\n## 事件列表\n")
	for i, event := range chunk {
		summary := strings.TrimSpace(event.Summary)
		if len([]rune(summary)) > 200 {
			summary = string([]rune(summary)[:200])
		}
		sb.WriteString(fmt.Sprintf("[%d] 标题:%s\n摘要:%s\n事件类型:%v\n", i, event.Title, summary, event.EventTypes))
		if len(event.Labels.Entities) > 0 {
			sb.WriteString("实体:")
			for _, entity := range event.Labels.Entities {
				sb.WriteString(fmt.Sprintf(" %s", entity.Name))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`## 输出 JSON
{"results":[{"news_index":0,"event_summary":"","items":[{"entity_name":"","entity_code":"","direction":"bullish|bearish|mixed|uncertain","impact_score":0.0,"impact_level":"weak|moderate|strong","confidence":0.0,"rationale":""}],"monitoring_signals":[{"signal":"","metric":"","operator":"gt|gte|lt|lte|eq","threshold":"","window":"","assets":[""],"reason":""}]}]}`)
	return sb.String()
}

func normalizeImpactMapping(mapping ImpactMapping, event *NewsEvent) ImpactMapping {
	if strings.TrimSpace(mapping.EventSummary) == "" {
		mapping.EventSummary = strings.TrimSpace(event.Summary)
		if mapping.EventSummary == "" {
			mapping.EventSummary = strings.TrimSpace(event.Title)
		}
	}

	for i := range mapping.Items {
		mapping.Items[i].ImpactScore = clampImpactScore(mapping.Items[i].ImpactScore)
		if mapping.Items[i].ImpactLevel == "" {
			mapping.Items[i].ImpactLevel = toImpactLevel(math.Abs(mapping.Items[i].ImpactScore))
		}
		if mapping.Items[i].Confidence <= 0 {
			mapping.Items[i].Confidence = 0.5
		}
		if mapping.Items[i].Direction == "" {
			mapping.Items[i].Direction = directionFromScore(mapping.Items[i].ImpactScore)
		}
	}

	if len(mapping.Items) == 0 {
		fallback := fallbackImpactMapping(event)
		mapping.Items = fallback.Items
		if len(mapping.MonitoringSignals) == 0 {
			mapping.MonitoringSignals = fallback.MonitoringSignals
		}
	}

	if len(mapping.MonitoringSignals) == 0 {
		mapping.MonitoringSignals = buildFallbackSignals(mapping)
	}
	return mapping
}

func fallbackImpactMapping(event *NewsEvent) ImpactMapping {
	mapping := ImpactMapping{
		EventSummary: strings.TrimSpace(event.Summary),
	}
	if mapping.EventSummary == "" {
		mapping.EventSummary = strings.TrimSpace(event.Title)
	}

	entities := event.Labels.Entities
	if len(entities) == 0 {
		entities = []EntityLabel{{
			Name:      guessPrimaryEntity(event),
			Role:      EntityRolePrimary,
			Relevance: 0.8,
		}}
	}

	score := clampImpactScore(event.Labels.Sentiment)
	if score == 0 {
		score = 0.2
	}
	for _, entity := range entities {
		if strings.TrimSpace(entity.Name) == "" {
			continue
		}
		item := ImpactItem{
			EntityName:  entity.Name,
			EntityCode:  entity.Code,
			Direction:   directionFromScore(score),
			ImpactScore: score,
			ImpactLevel: toImpactLevel(math.Abs(score)),
			Confidence:  maxFloat(entity.Relevance, 0.5),
			Rationale:   mapping.EventSummary,
		}
		mapping.Items = append(mapping.Items, item)
	}
	mapping.MonitoringSignals = buildFallbackSignals(mapping)
	return mapping
}

func buildFallbackSignals(mapping ImpactMapping) []StructuredMonitoringSignal {
	if len(mapping.Items) == 0 {
		return nil
	}
	item := mapping.Items[0]
	operator := SignalOperatorGreaterThan
	threshold := "0.4"
	if item.Direction == DirectionBearish {
		operator = SignalOperatorLessThan
		threshold = "-0.4"
	}
	asset := strings.TrimSpace(item.EntityName)
	if asset == "" {
		asset = "核心实体"
	}
	return []StructuredMonitoringSignal{{
		Signal:    fmt.Sprintf("%s情绪偏离", asset),
		Metric:    "news_sentiment",
		Operator:  operator,
		Threshold: threshold,
		Window:    "1d",
		Assets:    []string{asset},
		Reason:    mapping.EventSummary,
	}}
}

func guessPrimaryEntity(event *NewsEvent) string {
	title := strings.TrimSpace(event.Title)
	if title == "" {
		return "市场"
	}
	tokens := strings.Fields(title)
	if len(tokens) > 0 {
		return tokens[0]
	}
	runes := []rune(title)
	if len(runes) > 8 {
		return string(runes[:8])
	}
	return title
}

func toImpactLevel(absScore float64) ImpactLevel {
	switch {
	case absScore >= 0.7:
		return ImpactLevelStrong
	case absScore >= 0.35:
		return ImpactLevelModerate
	default:
		return ImpactLevelWeak
	}
}

func directionFromScore(score float64) Direction {
	switch {
	case score > 0.15:
		return DirectionBullish
	case score < -0.15:
		return DirectionBearish
	default:
		return DirectionUncertain
	}
}

func clampImpactScore(score float64) float64 {
	if score > 1 {
		return 1
	}
	if score < -1 {
		return -1
	}
	return score
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
