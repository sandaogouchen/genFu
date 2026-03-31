// Package news implements three-layer funnel filtering system.
// L1: Embedding semantic coarse filtering (anchor pool matching)
// L2: LLM batch relevance judgment
// L3: Deep causal chain analysis
package news

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// External Dependency Interfaces
// ──────────────────────────────────────────────

// EmbeddingService interface for vectorization service
type FunnelEmbeddingService interface {
	Encode(ctx context.Context, texts []string) ([][]float64, error)
}

// LLMService interface for LLM inference service
type LLMService interface {
	ChatComplete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// ──────────────────────────────────────────────
// Funnel Configuration
// ──────────────────────────────────────────────

// Config represents three-layer funnel configuration
type Config struct {
	// L1 configuration
	L1Threshold float64 `json:"l1_threshold"` // L1 weighted similarity threshold (default 0.40)
	L1TopN      int     `json:"l1_top_n"`     // L1 output TopN (default 30)

	// L2 configuration
	L2BatchSize    int       `json:"l2_batch_size"`    // L2 batch size (default 10)
	L2MinRelevance Relevance `json:"l2_min_relevance"` // L2 minimum relevance (default medium)

	// L3 configuration
	L3MaxAnalyze int `json:"l3_max_analyze"` // L3 max deep analysis count (default 5)

	// Risk-input upgrade switches
	EventImpactEnabled       bool    `json:"event_impact_enabled"`
	CausalVerifierEnabled    bool    `json:"causal_verifier_enabled"`
	EventImpactBatchSize     int     `json:"event_impact_batch_size"`
	VerifierMaxAnalyze       int     `json:"verifier_max_analyze"`
	VerifierWeakThreshold    float64 `json:"verifier_weak_threshold"`
	VerifierInvalidThreshold float64 `json:"verifier_invalid_threshold"`
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		L1Threshold:              0.40,
		L1TopN:                   30,
		L2BatchSize:              10,
		L2MinRelevance:           RelevanceMedium,
		L3MaxAnalyze:             5,
		EventImpactEnabled:       true,
		CausalVerifierEnabled:    true,
		EventImpactBatchSize:     10,
		VerifierMaxAnalyze:       5,
		VerifierWeakThreshold:    0.6,
		VerifierInvalidThreshold: 0.4,
	}
}

// ──────────────────────────────────────────────
// Funnel Main Body
// ──────────────────────────────────────────────

// Funnel represents three-layer funnel filter
type Funnel struct {
	config              Config
	embedSvc            FunnelEmbeddingService
	llmSvc              LLMService
	anchorPool          *AnchorPool
	portfolio           *PortfolioContext
	eventImpactAgent    EventImpactAgent
	causalVerifierAgent CausalVerifierAgent
}

// NewFunnel creates a three-layer funnel
func NewFunnel(embedSvc FunnelEmbeddingService, llmSvc LLMService, portfolio *PortfolioContext, opts ...FunnelOption) *Funnel {
	// 显式处理: 如果传入 nil 具体值，保持接口为 nil
	var svc FunnelEmbeddingService
	if embedSvc != nil {
		svc = embedSvc
	}
	log.Printf("[Funnel] 初始化 embedSvc=%v nil=%v", svc, svc == nil)
	f := &Funnel{
		config:     DefaultConfig(),
		embedSvc:   svc,
		llmSvc:     llmSvc,
		anchorPool: NewAnchorPool(),
		portfolio:  portfolio,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

type FunnelOption func(*Funnel)

func WithFunnelConfig(c Config) FunnelOption {
	return func(f *Funnel) { f.config = c }
}

func WithEventImpactAgent(agent EventImpactAgent) FunnelOption {
	return func(f *Funnel) { f.eventImpactAgent = agent }
}

func WithCausalVerifierAgent(agent CausalVerifierAgent) FunnelOption {
	return func(f *Funnel) { f.causalVerifierAgent = agent }
}

// ──────────────────────────────────────────────
// Anchor Pool Management
// ──────────────────────────────────────────────

// AnchorPool represents anchor pool
type AnchorPool struct {
	anchors   []Anchor
	updatedAt time.Time
}

// NewAnchorPool creates a new anchor pool
func NewAnchorPool() *AnchorPool {
	return &AnchorPool{}
}

// BuildFromPortfolio builds anchor pool from portfolio
func (ap *AnchorPool) BuildFromPortfolio(ctx context.Context, embedSvc FunnelEmbeddingService, portfolio *PortfolioContext) error {
	// If embedding service is not available, create anchors without embeddings
	// (This means L1 filtering will be skipped)
	if embedSvc == nil {
		var anchors []Anchor

		// Holding direct anchors
		for _, h := range portfolio.Holdings {
			anchors = append(anchors, Anchor{
				ID:           fmt.Sprintf("hold_%s", h.Code),
				Type:         AnchorHoldingDirect,
				Text:         h.Name,
				Weight:       AnchorWeights[AnchorHoldingDirect],
				RelatedAsset: h.Name,
			})

			// Holding product anchors
			for _, p := range h.Products {
				anchors = append(anchors, Anchor{
					ID:           fmt.Sprintf("prod_%s_%s", h.Code, sanitize(p)),
					Type:         AnchorHoldingProduct,
					Text:         p,
					Weight:       AnchorWeights[AnchorHoldingProduct],
					RelatedAsset: h.Name,
				})
			}

			// Holding competitor anchors
			for _, c := range h.Competitors {
				anchors = append(anchors, Anchor{
					ID:           fmt.Sprintf("comp_%s_%s", h.Code, sanitize(c)),
					Type:         AnchorHoldingCompetitor,
					Text:         c,
					Weight:       AnchorWeights[AnchorHoldingCompetitor],
					RelatedAsset: h.Name,
				})
			}

			// Supply chain anchors
			for _, s := range h.SupplyChain {
				anchors = append(anchors, Anchor{
					ID:           fmt.Sprintf("supply_%s_%s", h.Code, sanitize(s)),
					Type:         AnchorHoldingSupply,
					Text:         s,
					Weight:       AnchorWeights[AnchorHoldingSupply],
					RelatedAsset: h.Name,
				})
			}
		}

		// Watchlist anchors
		for _, w := range portfolio.Watchlist {
			anchors = append(anchors, Anchor{
				ID:           fmt.Sprintf("watch_%s", w.Code),
				Type:         AnchorWatchlist,
				Text:         w.Name,
				Weight:       AnchorWeights[AnchorWatchlist],
				RelatedAsset: w.Name,
			})
		}

		// Industry theme anchors
		for i, theme := range portfolio.IndustryThemes {
			anchors = append(anchors, Anchor{
				ID:     fmt.Sprintf("theme_%d", i),
				Type:   AnchorIndustryTheme,
				Text:   theme,
				Weight: AnchorWeights[AnchorIndustryTheme],
			})
		}

		// Macro factor anchors
		for i, factor := range portfolio.MacroFactors {
			anchors = append(anchors, Anchor{
				ID:     fmt.Sprintf("macro_%d", i),
				Type:   AnchorMacroFactor,
				Text:   factor,
				Weight: AnchorWeights[AnchorMacroFactor],
			})
		}

		ap.anchors = anchors
		ap.updatedAt = time.Now()
		return nil
	}

	// Normal path with embedding service
	var anchors []Anchor
	var textsToEncode []string
	var pendingAnchors []Anchor

	// Holding direct anchors
	for _, h := range portfolio.Holdings {
		a := Anchor{
			ID:           fmt.Sprintf("hold_%s", h.Code),
			Type:         AnchorHoldingDirect,
			Text:         h.Name,
			Weight:       AnchorWeights[AnchorHoldingDirect],
			RelatedAsset: h.Name,
		}
		textsToEncode = append(textsToEncode, h.Name)
		pendingAnchors = append(pendingAnchors, a)

		// Holding product anchors
		for _, p := range h.Products {
			a := Anchor{
				ID:           fmt.Sprintf("prod_%s_%s", h.Code, sanitize(p)),
				Type:         AnchorHoldingProduct,
				Text:         p,
				Weight:       AnchorWeights[AnchorHoldingProduct],
				RelatedAsset: h.Name,
			}
			textsToEncode = append(textsToEncode, p)
			pendingAnchors = append(pendingAnchors, a)
		}

		// Holding competitor anchors
		for _, c := range h.Competitors {
			a := Anchor{
				ID:           fmt.Sprintf("comp_%s_%s", h.Code, sanitize(c)),
				Type:         AnchorHoldingCompetitor,
				Text:         c,
				Weight:       AnchorWeights[AnchorHoldingCompetitor],
				RelatedAsset: h.Name,
			}
			textsToEncode = append(textsToEncode, c)
			pendingAnchors = append(pendingAnchors, a)
		}

		// Holding supply chain anchors
		for _, s := range h.SupplyChain {
			a := Anchor{
				ID:           fmt.Sprintf("supply_%s_%s", h.Code, sanitize(s)),
				Type:         AnchorHoldingSupply,
				Text:         s,
				Weight:       AnchorWeights[AnchorHoldingSupply],
				RelatedAsset: h.Name,
			}
			textsToEncode = append(textsToEncode, s)
			pendingAnchors = append(pendingAnchors, a)
		}
	}

	// Watchlist anchors
	for _, w := range portfolio.Watchlist {
		a := Anchor{
			ID:           fmt.Sprintf("watch_%s", sanitize(w.Name)),
			Type:         AnchorWatchlist,
			Text:         w.Name,
			Weight:       AnchorWeights[AnchorWatchlist],
			RelatedAsset: w.Name,
		}
		textsToEncode = append(textsToEncode, w.Name)
		pendingAnchors = append(pendingAnchors, a)
	}

	// Industry theme anchors
	for _, theme := range portfolio.IndustryThemes {
		a := Anchor{
			ID:           fmt.Sprintf("theme_%s", sanitize(theme)),
			Type:         AnchorIndustryTheme,
			Text:         theme,
			Weight:       AnchorWeights[AnchorIndustryTheme],
			RelatedAsset: theme,
		}
		textsToEncode = append(textsToEncode, theme)
		pendingAnchors = append(pendingAnchors, a)
	}

	// Macro factor anchors
	for _, factor := range portfolio.MacroFactors {
		a := Anchor{
			ID:           fmt.Sprintf("macro_%s", sanitize(factor)),
			Type:         AnchorMacroFactor,
			Text:         factor,
			Weight:       AnchorWeights[AnchorMacroFactor],
			RelatedAsset: factor,
		}
		textsToEncode = append(textsToEncode, factor)
		pendingAnchors = append(pendingAnchors, a)
	}

	// Fixed configuration: General risk anchors
	riskAnchors := []struct{ text, id string }{
		{"系统性风险 黑天鹅 流动性危机", "risk_systemic"},
		{"金融危机 恐慌 崩盘 暴跌", "risk_crisis"},
	}
	for _, ra := range riskAnchors {
		a := Anchor{
			ID:           ra.id,
			Type:         AnchorGeneralRisk,
			Text:         ra.text,
			Weight:       AnchorWeights[AnchorGeneralRisk],
			RelatedAsset: "全市场",
		}
		textsToEncode = append(textsToEncode, ra.text)
		pendingAnchors = append(pendingAnchors, a)
	}

	// Fixed configuration: Safe haven signal anchors
	havenAnchors := []struct{ text, id string }{
		{"地缘冲突 战争 制裁 能源危机", "haven_geopolitical"},
		{"避险 黄金 美债 日元", "haven_assets"},
	}
	for _, ha := range havenAnchors {
		a := Anchor{
			ID:           ha.id,
			Type:         AnchorSafeHaven,
			Text:         ha.text,
			Weight:       AnchorWeights[AnchorSafeHaven],
			RelatedAsset: "避险资产",
		}
		textsToEncode = append(textsToEncode, ha.text)
		pendingAnchors = append(pendingAnchors, a)
	}

	// Batch encode
	if len(textsToEncode) > 0 {
		embeddings, err := embedSvc.Encode(ctx, textsToEncode)
		if err != nil {
			return fmt.Errorf("encoding anchor texts: %w", err)
		}
		for i := range pendingAnchors {
			pendingAnchors[i].Embedding = embeddings[i]
			pendingAnchors[i].UpdatedAt = time.Now()
		}
	}

	anchors = append(anchors, pendingAnchors...)
	ap.anchors = anchors
	ap.updatedAt = time.Now()
	return nil
}

// ──────────────────────────────────────────────
// Filter Main Entry
// ──────────────────────────────────────────────

// Filter executes three-layer funnel filtering
func (f *Funnel) Filter(ctx context.Context, events []*NewsEvent) ([]*NewsEvent, error) {
	// 使用反射检查接口内部是否为 nil
	isNil := f.embedSvc == nil
	if !isNil {
		v := reflect.ValueOf(f.embedSvc)
		if v.Kind() == reflect.Ptr && v.IsNil() {
			isNil = true
		}
	}

	log.Printf("[Funnel.Filter] 检查 embedSvc nil=%v len(events)=%d", isNil, len(events))

	// If embedding service is not available, skip L1 and use fallback strategy
	if isNil {
		log.Printf("[Funnel.Filter] 降级模式: 跳过L1，直接使用L2")
		// Fallback: skip L1, directly use L2 for all events
		l2Passed, err := f.layerTwo(ctx, events)
		if err != nil {
			// If L2 also fails, return all events (only keyword-filtered)
			return events, nil
		}

		// L3: Deep causal chain analysis (only for items marked as needs_deep)
		_ = f.layerThree(ctx, l2Passed) // Error doesn't affect return

		return l2Passed, nil
	}

	// Ensure anchor pool is initialized
	if len(f.anchorPool.anchors) == 0 && f.portfolio != nil {
		if err := f.anchorPool.BuildFromPortfolio(ctx, f.embedSvc, f.portfolio); err != nil {
			return nil, fmt.Errorf("building anchor pool: %w", err)
		}
	}

	// L1: Embedding semantic coarse filtering
	l1Passed, err := f.layerOne(ctx, events)
	if err != nil {
		return nil, fmt.Errorf("L1 filtering: %w", err)
	}

	// L2: LLM batch relevance judgment
	l2Passed, err := f.layerTwo(ctx, l1Passed)
	if err != nil {
		// L2 failure falls back to using all L1 results
		return l1Passed, nil
	}

	// L3: Deep causal chain analysis (only for items marked as needs_deep)
	err = f.layerThree(ctx, l2Passed)
	if err != nil {
		// L3 failure doesn't affect return
	}

	return l2Passed, nil
}

// ──────────────────────────────────────────────
// Layer 1: Embedding Semantic Coarse Filtering
// ──────────────────────────────────────────────

func (f *Funnel) layerOne(ctx context.Context, events []*NewsEvent) ([]*NewsEvent, error) {
	if len(events) == 0 {
		return nil, nil
	}

	// Encode news texts
	texts := make([]string, len(events))
	for i, e := range events {
		summary := e.Summary
		if len([]rune(summary)) > 150 {
			summary = string([]rune(summary)[:150])
		}
		texts[i] = e.Title + "。" + summary
	}

	newsEmbeddings, err := f.embedSvc.Encode(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("encoding news texts: %w", err)
	}

	// Calculate anchor matches for each news
	type scored struct {
		event *NewsEvent
		score float64
	}
	var scoredEvents []scored

	for i, event := range events {
		if i >= len(newsEmbeddings) {
			break
		}
		newsEmb := newsEmbeddings[i]
		bestScore := 0.0
		var matchedAnchors []AnchorMatch

		for _, anchor := range f.anchorPool.anchors {
			sim := cosineSimilarityFloat64(newsEmb, anchor.Embedding)
			weightedScore := sim * anchor.Weight

			if weightedScore > f.config.L1Threshold {
				matchedAnchors = append(matchedAnchors, AnchorMatch{
					AnchorID:      anchor.ID,
					AnchorText:    anchor.Text,
					AnchorType:    anchor.Type,
					Similarity:    sim,
					WeightedScore: weightedScore,
					RelatedAsset:  anchor.RelatedAsset,
				})
			}

			if weightedScore > bestScore {
				bestScore = weightedScore
			}
		}

		if bestScore > f.config.L1Threshold {
			// Initialize FunnelResult
			if event.FunnelResult == nil {
				event.FunnelResult = &FunnelResult{}
			}
			event.FunnelResult.L1Score = bestScore
			event.FunnelResult.L1MatchedAnchors = matchedAnchors
			event.FunnelResult.L1Pass = true

			// Collect affected assets
			assetSet := make(map[string]bool)
			for _, m := range matchedAnchors {
				assetSet[m.RelatedAsset] = true
			}

			scoredEvents = append(scoredEvents, scored{event: event, score: bestScore})
		}
	}

	// Sort by L1 score descending, take Top N
	sort.Slice(scoredEvents, func(i, j int) bool {
		return scoredEvents[i].score > scoredEvents[j].score
	})

	topN := f.config.L1TopN
	if topN > len(scoredEvents) {
		topN = len(scoredEvents)
	}

	result := make([]*NewsEvent, topN)
	for i := 0; i < topN; i++ {
		result[i] = scoredEvents[i].event
	}

	return result, nil
}

// ──────────────────────────────────────────────
// Layer 2: LLM Batch Relevance Judgment
// ──────────────────────────────────────────────

// L2Response represents LLM L2 output structure
type L2Response struct {
	Results []L2Result `json:"results"`
}

type L2Result struct {
	NewsIndex      int             `json:"news_index"`
	Relevance      Relevance       `json:"relevance"`
	AffectedAssets []L2AssetImpact `json:"affected_assets"`
	CausalSketch   string          `json:"causal_sketch"`
	Priority       int             `json:"priority"`
	NeedsDeep      bool            `json:"needs_deep_analysis"`
}

type L2AssetImpact struct {
	Asset     string    `json:"asset"`
	Code      string    `json:"code,omitempty"`
	Direction Direction `json:"direction"`
}

func (f *Funnel) layerTwo(ctx context.Context, events []*NewsEvent) ([]*NewsEvent, error) {
	if len(events) == 0 {
		return events, nil
	}
	if f.llmSvc == nil && (!f.config.EventImpactEnabled || f.eventImpactAgent == nil) {
		return events, nil
	}

	var passed []*NewsEvent

	// Process in batches
	for batchStart := 0; batchStart < len(events); batchStart += f.config.L2BatchSize {
		batchEnd := batchStart + f.config.L2BatchSize
		if batchEnd > len(events) {
			batchEnd = len(events)
		}
		batch := events[batchStart:batchEnd]

		batchPassed, err := f.processL2Batch(ctx, batch)
		if err != nil {
			// Batch failure keeps all
			passed = append(passed, batch...)
			continue
		}
		passed = append(passed, batchPassed...)
	}

	return passed, nil
}

func (f *Funnel) processL2Batch(ctx context.Context, batch []*NewsEvent) ([]*NewsEvent, error) {
	var l2Err error
	var l2Resp L2Response

	if f.llmSvc != nil {
		// Build System Prompt
		systemPrompt := f.buildL2SystemPrompt()
		userPrompt := f.buildL2UserPrompt(batch)

		// Call LLM
		response, err := f.llmSvc.ChatComplete(ctx, systemPrompt, userPrompt)
		if err != nil {
			l2Err = fmt.Errorf("L2 LLM call failed: %w", err)
		} else {
			// Parse JSON response
			jsonStr := extractJSON(response)
			if err := json.Unmarshal([]byte(jsonStr), &l2Resp); err != nil {
				l2Err = fmt.Errorf("parsing L2 LLM response: %w", err)
			}
		}
	}

	// Apply L2 results (if available)
	resultMap := make(map[int]L2Result)
	for _, r := range l2Resp.Results {
		resultMap[r.NewsIndex] = r
	}

	impactMappings := make([]ImpactMapping, len(batch))
	for i, event := range batch {
		impactMappings[i] = fallbackImpactMapping(event)
	}
	if f.config.EventImpactEnabled && f.eventImpactAgent != nil {
		mappings, err := f.eventImpactAgent.AnalyzeBatch(ctx, batch, f.portfolio)
		if err != nil {
			log.Printf("[Funnel.L2] EventImpactAgent fallback used: %v", err)
		}
		for i := range batch {
			if i < len(mappings) {
				impactMappings[i] = normalizeImpactMapping(mappings[i], batch[i])
			}
		}
	}

	if l2Err != nil && !f.config.EventImpactEnabled {
		return nil, l2Err
	}

	var passed []*NewsEvent
	for i, event := range batch {
		r, ok := resultMap[i]
		if event.FunnelResult == nil {
			event.FunnelResult = &FunnelResult{}
		}
		if ok {
			event.FunnelResult.L2Relevance = r.Relevance
			event.FunnelResult.L2CausalSketch = r.CausalSketch
			event.FunnelResult.L2Priority = r.Priority
			event.FunnelResult.L2NeedsDeep = r.NeedsDeep

			// Convert affected assets
			event.FunnelResult.L2AffectedAssets = event.FunnelResult.L2AffectedAssets[:0]
			for _, a := range r.AffectedAssets {
				isHolding := f.isHolding(a.Asset)
				event.FunnelResult.L2AffectedAssets = append(event.FunnelResult.L2AffectedAssets, AssetImpact{
					AssetName: a.Asset,
					AssetCode: a.Code,
					Direction: a.Direction,
					IsHolding: isHolding,
				})
			}
		}

		// New risk-input path: entity/impact/exposure/monitoring mapping.
		if f.config.EventImpactEnabled {
			mapping := impactMappings[i]
			exposures := BuildExposureMapping(event, mapping, f.portfolio)
			backfillLegacyFromRiskMapping(event, mapping, exposures)
		}

		// Determine if passes L2
		if event.FunnelResult.L2Relevance == RelevanceHigh || event.FunnelResult.L2Relevance == RelevanceMedium {
			event.FunnelResult.L2Pass = true
			passed = append(passed, event)
		}
	}

	if len(passed) == 0 && l2Err != nil {
		// Keep batch for resilience when L2 fails.
		return batch, nil
	}

	return passed, nil
}

func (f *Funnel) buildL2SystemPrompt() string {
	return `你是一个投资组合新闻关联度分析引擎。
你的任务是判断一批新闻与给定投资组合的关联程度。

判断标准:
- high: 新闻直接影响持仓资产，需立即关注
- medium: 新闻间接影响持仓或影响关注的行业/主题
- low: 存在间接关联但影响较小
- none: 与投资组合无关

needs_deep_analysis = true 的触发条件:
1. 存在非显而易见的因果传导路径
2. 影响多个持仓资产且方向不一致
3. 事件为 breaking 且 priority ≥ 4

输出严格 JSON 格式。`
}

func (f *Funnel) buildL2UserPrompt(batch []*NewsEvent) string {
	var sb strings.Builder

	// Part 1: Investment context
	sb.WriteString("## 投资组合上下文\n\n### 持仓:\n")
	if f.portfolio != nil {
		for _, h := range f.portfolio.Holdings {
			sb.WriteString(fmt.Sprintf("- %s (%s) | 行业: %s | 仓位: %.1f%%\n", h.Name, h.Code, h.Industry, h.Weight*100))
		}
		sb.WriteString("\n### 关注列表:\n")
		for _, w := range f.portfolio.Watchlist {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", w.Name, w.Code))
		}
		sb.WriteString("\n### 关注主题:\n")
		for _, t := range f.portfolio.IndustryThemes {
			sb.WriteString(fmt.Sprintf("- %s\n", t))
		}
	}

	// Part 2: News to judge
	sb.WriteString("\n## 待判断新闻\n\n")
	for i, event := range batch {
		summary := event.Summary
		if len([]rune(summary)) > 200 {
			summary = string([]rune(summary)[:200])
		}
		sb.WriteString(fmt.Sprintf("[%d] 事件类型: %v | 标题: %s\n摘要: %s\n",
			i, event.EventTypes, event.Title, summary))
		if event.FunnelResult != nil && len(event.FunnelResult.L1MatchedAnchors) > 0 {
			sb.WriteString("L1匹配锚点: ")
			for _, m := range event.FunnelResult.L1MatchedAnchors {
				sb.WriteString(fmt.Sprintf("%s(%.2f) ", m.AnchorText, m.WeightedScore))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Part 3: Output requirements
	sb.WriteString(`## 输出要求
输出严格 JSON，格式如下:
{"results": [{"news_index": 0, "relevance": "high|medium|low|none", "affected_assets": [{"asset": "名称", "code": "代码", "direction": "bullish|bearish|mixed|uncertain"}], "causal_sketch": "≤40字因果逻辑", "priority": 1-5, "needs_deep_analysis": true|false}]}`)

	return sb.String()
}

// ──────────────────────────────────────────────
// Layer 3: Deep Causal Chain Analysis
// ──────────────────────────────────────────────

func (f *Funnel) layerThree(ctx context.Context, events []*NewsEvent) error {
	// Causal verification first (degrade but do not block).
	if f.config.CausalVerifierEnabled && f.causalVerifierAgent != nil {
		candidates := make([]*NewsEvent, 0)
		for _, event := range events {
			if event.FunnelResult != nil && event.FunnelResult.L2Pass {
				candidates = append(candidates, event)
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].FunnelResult.L2Priority > candidates[j].FunnelResult.L2Priority
		})
		limit := f.config.VerifierMaxAnalyze
		if limit <= 0 {
			limit = f.config.L3MaxAnalyze
		}
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}

		if len(candidates) > 0 {
			verifications, err := f.causalVerifierAgent.VerifyBatch(ctx, candidates, f.portfolio)
			if err != nil {
				log.Printf("[Funnel.L3] CausalVerifierAgent fallback used: %v", err)
			}
			for i, event := range candidates {
				if i < len(verifications) {
					applyVerificationPenalty(event, verifications[i])
				}
			}
		}
	}

	if f.llmSvc == nil {
		return nil
	}

	// Filter items needing deep analysis
	var needsDeep []*NewsEvent
	for _, e := range events {
		if e.FunnelResult != nil && e.FunnelResult.L2NeedsDeep {
			needsDeep = append(needsDeep, e)
		}
	}

	// Limit count
	if len(needsDeep) > f.config.L3MaxAnalyze {
		needsDeep = needsDeep[:f.config.L3MaxAnalyze]
	}

	// Deep analyze each item
	for _, event := range needsDeep {
		analysis, err := f.deepAnalyze(ctx, event)
		if err != nil {
			continue
		}
		event.FunnelResult.L3Analysis = analysis
		if len(event.FunnelResult.MonitoringSignalsV2) == 0 && len(analysis.MonitoringSignals) > 0 {
			event.FunnelResult.MonitoringSignalsV2 = convertLegacyMonitoringSignals(event, analysis.MonitoringSignals)
		}
		backfillLegacyMonitoringSignals(event)
	}

	return nil
}

func (f *Funnel) deepAnalyze(ctx context.Context, event *NewsEvent) (*CausalAnalysis, error) {
	systemPrompt := `你是一个专业的投资因果链分析引擎。
你的任务是对一条新闻事件进行深度因果链分析。

分析原则:
1. 逐跳验证：每一跳必须有独立的经济学/行业逻辑支撑
2. 置信度公式：confidence = 逻辑严密性(40%) + 历史一致性(30%) + 当前适用性(30%)
3. 低置信标注：任何 hop 的 confidence < 0.4 必须在 uncertainties 中说明
4. 强制反面论据：counter_chains 不能为空，必须主动寻找反面逻辑
5. 跨资产发现：cross_asset_impacts 是选股信号的关键来源

输出严格 JSON 格式。`

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 事件\n标题: %s\n摘要: %s\n事件类型: %v\n\n",
		event.Title, event.Summary, event.EventTypes))

	if event.FunnelResult != nil {
		sb.WriteString(fmt.Sprintf("L2因果草图: %s\n", event.FunnelResult.L2CausalSketch))
		sb.WriteString("L2影响资产:\n")
		for _, a := range event.FunnelResult.L2AffectedAssets {
			sb.WriteString(fmt.Sprintf("- %s: %s (持仓: %v)\n", a.AssetName, a.Direction, a.IsHolding))
		}
	}

	sb.WriteString(`
## 输出JSON结构
{"event_summary": "一句话总结", "causal_chains": [{"hops": [{"from": "起点", "to": "终点", "mechanism": "传导机制", "confidence": 0.0-1.0, "evidence": "历史案例/依据", "time_lag": "时滞"}], "final_impact": {"asset": "资产", "direction": "bullish/bearish", "magnitude": "high/medium/low", "timeframe": "时间框架"}}], "counter_chains": [...], "key_uncertainties": [...], "monitoring_signals": [...], "cross_asset_impacts": [{"asset": "资产", "asset_code": "代码", "direction": "方向", "mechanism": "机制", "confidence": 0.0-1.0, "is_holding": false, "source_event": "源事件"}]}`)

	response, err := f.llmSvc.ChatComplete(ctx, systemPrompt, sb.String())
	if err != nil {
		return nil, fmt.Errorf("L3 LLM call failed: %w", err)
	}

	var analysis CausalAnalysis
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		return nil, fmt.Errorf("parsing L3 response: %w", err)
	}

	return &analysis, nil
}

// ──────────────────────────────────────────────
// Helper Functions
// ──────────────────────────────────────────────

func (f *Funnel) isHolding(assetName string) bool {
	if f.portfolio == nil {
		return false
	}
	name := strings.ToLower(assetName)
	for _, h := range f.portfolio.Holdings {
		if strings.Contains(strings.ToLower(h.Name), name) || strings.Contains(name, strings.ToLower(h.Name)) {
			return true
		}
	}
	return false
}

func cosineSimilarityFloat64(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func sanitize(s string) string {
	r := strings.NewReplacer(" ", "_", "/", "_", ".", "_")
	return r.Replace(strings.ToLower(s))
}

// extractJSON extracts JSON from LLM response (compatible with markdown code block)
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Try to extract ```json ... ``` block
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
		if endIdx := strings.Index(s, "```"); endIdx >= 0 {
			s = s[:endIdx]
		}
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if endIdx := strings.Index(s, "```"); endIdx >= 0 {
			s = s[:endIdx]
		}
	}
	// Find first { and last }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		s = s[start : end+1]
	}
	return strings.TrimSpace(s)
}
