// Package news defines core domain models for news analysis system.
// Based on RavenPack 5-layer taxonomy (~7,400 event types) and LSEG/Refinitiv 2,500+ topic codes,
// designed for Chinese market with 7 major event domains × multi-dimensional labels.
package news

import "time"

// ──────────────────────────────────────────────
// Level 1 Event Domains (7 major domains)
// ──────────────────────────────────────────────

// EventDomain represents level 1 event domain enumeration
type EventDomain string

const (
	DomainMacro        EventDomain = "macro"        // Macroeconomics: central banks, economic data, fiscal policy
	DomainGeopolitical EventDomain = "geopolitical" // Geopolitics: military conflicts, sanctions, diplomacy
	DomainIndustry     EventDomain = "industry"     // Industry: technology breakthroughs, supply chain, capacity
	DomainCorporate    EventDomain = "corporate"    // Corporate: earnings, M&A, management, products
	DomainRegulatory   EventDomain = "regulatory"   // Regulatory: policy releases, investigations, approvals
	DomainMarket       EventDomain = "market"       // Market behavior: analyst ratings, fund flows, anomalies
	DomainTechnology   EventDomain = "technology"   // Technology: AI models, chips, biotech
)

// AllDomains contains all level 1 event domains
var AllDomains = []EventDomain{
	DomainMacro, DomainGeopolitical, DomainIndustry,
	DomainCorporate, DomainRegulatory, DomainMarket, DomainTechnology,
}

// DomainMeta represents event domain metadata
type DomainMeta struct {
	Code        EventDomain
	Name        string
	Description string
	Examples    []string
}

// DomainRegistry is the event domain metadata registry
var DomainRegistry = map[EventDomain]DomainMeta{
	DomainMacro:        {DomainMacro, "宏观经济", "央行、经济数据、财政货币政策", []string{"美联储加息", "CPI数据"}},
	DomainGeopolitical: {DomainGeopolitical, "地缘政治", "军事冲突、制裁、外交", []string{"美国攻打伊朗", "中美关税"}},
	DomainIndustry:     {DomainIndustry, "行业动态", "技术突破、供应链、产能", []string{"Seedance 2.0发布"}},
	DomainCorporate:    {DomainCorporate, "公司事件", "财报、并购、管理层、产品", []string{"字节跳动IPO传闻"}},
	DomainRegulatory:   {DomainRegulatory, "监管政策", "政策发布、调查处罚、审批", []string{"游戏版号发放"}},
	DomainMarket:       {DomainMarket, "市场行为", "分析师评级、资金流向、异动", []string{"北向资金大幅流出"}},
	DomainTechnology:   {DomainTechnology, "技术突破", "AI模型、芯片、生物技术", []string{"GPT-5发布"}},
}

// ──────────────────────────────────────────────
// Level 2 Event Types
// ──────────────────────────────────────────────

// EventType represents level 2 event type
type EventType struct {
	Domain       EventDomain `json:"domain"`
	TypeCode     string      `json:"type_code"`
	TypeName     string      `json:"type_name"`
	Keywords     []string    `json:"keywords"`
	TypicalAsset string      `json:"typical_asset,omitempty"`
}

// ──────────────────────────────────────────────
// News Source Types
// ──────────────────────────────────────────────

// SourceType represents news source type
type SourceType string

const (
	SourceFinancialMedia SourceType = "financial_media" // Financial media
	SourceOfficialGov    SourceType = "official_gov"    // Official government
	SourceSocialMedia    SourceType = "social_media"    // Social media
	SourceDataProvider   SourceType = "data_provider"   // Data terminals
)

// ──────────────────────────────────────────────
// Raw News (before processing)
// ──────────────────────────────────────────────

// RawNews represents a raw news item (after collection, before processing)
type RawNews struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Content        string     `json:"content"`
	Source         string     `json:"source"`
	SourceType     SourceType `json:"source_type"`
	URL            string     `json:"url"`
	PublishedAt    time.Time  `json:"published_at"`
	CollectedAt    time.Time  `json:"collected_at"`
	Language       string     `json:"language"` // "zh" / "en"
	TitleHash      string     `json:"title_hash"`
	RelatedSources []string   `json:"related_sources,omitempty"`
}

// ──────────────────────────────────────────────
// Processed News Event (Pipeline core structure)
// ──────────────────────────────────────────────

// NewsEvent represents a classified and labeled news event
type NewsEvent struct {
	// Basic info
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Content     string     `json:"content"`
	Source      string     `json:"source"`
	SourceType  SourceType `json:"source_type"`
	URL         string     `json:"url"`
	PublishedAt time.Time  `json:"published_at"`
	ProcessedAt time.Time  `json:"processed_at"`

	// Classification info (multi-label)
	Domains    []EventDomain `json:"domains"`     // Level 1 domains (can be multiple)
	EventTypes []string      `json:"event_types"` // Level 2 event types
	Labels     LabelSet      `json:"labels"`      // Multi-dimensional labels

	// Embedding
	Embedding []float64 `json:"embedding,omitempty"`

	// Classification confidence
	ClassifyConfidence float64 `json:"classify_confidence"`
	ClassifyMethod     string  `json:"classify_method"` // "keyword" / "embedding" / "llm"

	// Dedup info
	DedupClusterID string   `json:"dedup_cluster_id,omitempty"`
	RelatedSources []string `json:"related_sources,omitempty"`

	// Funnel layer results
	FunnelResult *FunnelResult `json:"funnel_result,omitempty"`

	// Database ID
	DBID int64 `json:"db_id,omitempty"`
}

// ──────────────────────────────────────────────
// Multi-dimensional Label System
// ───────────────────────────────────────���──────

// LabelSet represents complete multi-dimensional label set
type LabelSet struct {
	Sentiment      float64        `json:"sentiment"`      // Sentiment score -1.0 ~ +1.0
	Novelty        Novelty        `json:"novelty"`        // Novelty
	Predictability Predictability `json:"predictability"` // Predictability
	Timeframe      Timeframe      `json:"timeframe"`      // Timeframe
	Entities       []EntityLabel  `json:"entities"`       // Entity labels
}

// SentimentLevel maps continuous sentiment score to discrete level
type SentimentLevel string

const (
	SentimentVeryNegative SentimentLevel = "very_negative" // < -0.6
	SentimentNegative     SentimentLevel = "negative"      // -0.6 ~ -0.2
	SentimentNeutral      SentimentLevel = "neutral"       // -0.2 ~ +0.2
	SentimentPositive     SentimentLevel = "positive"      // +0.2 ~ +0.6
	SentimentVeryPositive SentimentLevel = "very_positive" // > +0.6
)

// ToSentimentLevel converts sentiment score to discrete level
func ToSentimentLevel(score float64) SentimentLevel {
	switch {
	case score < -0.6:
		return SentimentVeryNegative
	case score < -0.2:
		return SentimentNegative
	case score < 0.2:
		return SentimentNeutral
	case score < 0.6:
		return SentimentPositive
	default:
		return SentimentVeryPositive
	}
}

// Novelty represents news novelty
type Novelty string

const (
	NoveltyBreaking  Novelty = "breaking"  // Breaking news
	NoveltyFollowUp  Novelty = "follow_up" // Follow-up report
	NoveltyRecurring Novelty = "recurring" // Recurring news
	NoveltyOldNews   Novelty = "old_news"  // Old news
)

// Predictability represents predictability
type Predictability string

const (
	PredictabilityScheduled   Predictability = "scheduled"   // Scheduled (e.g., FOMC meeting)
	PredictabilityUnscheduled Predictability = "unscheduled" // Unexpected event
	PredictabilitySemiKnown   Predictability = "semi_known"  // Semi-expected (market rumors)
)

// Timeframe represents time framework
type Timeframe string

const (
	TimeframeImmediate Timeframe = "immediate" // Immediate (intraday reaction)
	TimeframeShort     Timeframe = "short"     // Short term (1-3 days)
	TimeframeMedium    Timeframe = "medium"    // Medium term (1-4 weeks)
	TimeframeLong      Timeframe = "long"      // Long term (1 month+)
)

// EntityLabel represents entity role in news
type EntityLabel struct {
	Name      string     `json:"name"`
	Code      string     `json:"code,omitempty"`
	Role      EntityRole `json:"role"`
	Relevance float64    `json:"relevance"` // 0-1.0 who the news is really "about"
}

// EntityRole represents entity role
type EntityRole string

const (
	EntityRolePrimary   EntityRole = "primary"   // Main subject of the news
	EntityRoleSecondary EntityRole = "secondary" // Secondary related
	EntityRoleMentioned EntityRole = "mentioned" // Just mentioned
)

// ──────────────────────────────────────────────
// Funnel Filter Results
// ──────────────────────────────────────────────

// FunnelResult represents three-layer funnel comprehensive output
type FunnelResult struct {
	// L1: Embedding semantic coarse filtering
	L1Score          float64       `json:"l1_score"`
	L1MatchedAnchors []AnchorMatch `json:"l1_matched_anchors"`
	L1Pass           bool          `json:"l1_pass"`

	// L2: LLM batch relevance judgment
	L2Relevance      Relevance     `json:"l2_relevance"`
	L2AffectedAssets []AssetImpact `json:"l2_affected_assets"`
	L2CausalSketch   string        `json:"l2_causal_sketch"`
	L2Priority       int           `json:"l2_priority"` // 1-5
	L2NeedsDeep      bool          `json:"l2_needs_deep"`
	L2Pass           bool          `json:"l2_pass"`

	// L3: Deep causal chain analysis
	L3Analysis *CausalAnalysis `json:"l3_analysis,omitempty"`

	// Risk-input upgrade fields (backward compatible extension)
	EventEntities       []EntityLabel                `json:"event_entities,omitempty"`
	ImpactMapping       *ImpactMapping               `json:"impact_mapping,omitempty"`
	ExposureMapping     []PortfolioExposure          `json:"exposure_mapping,omitempty"`
	CausalVerification  *CausalVerification          `json:"causal_verification,omitempty"`
	MonitoringSignalsV2 []StructuredMonitoringSignal `json:"monitoring_signals_v2,omitempty"`
}

// Relevance represents L2 relevance level
type Relevance string

const (
	RelevanceHigh   Relevance = "high"
	RelevanceMedium Relevance = "medium"
	RelevanceLow    Relevance = "low"
	RelevanceNone   Relevance = "none"
)

// AssetImpact represents asset impact assessment
type AssetImpact struct {
	AssetName    string    `json:"asset_name"`
	AssetCode    string    `json:"asset_code,omitempty"`
	Direction    Direction `json:"direction"`
	Confidence   float64   `json:"confidence"`
	IsHolding    bool      `json:"is_holding"`
	CausalSketch string    `json:"causal_sketch,omitempty"`
}

// Direction represents direction judgment
type Direction string

const (
	DirectionBullish   Direction = "bullish"
	DirectionBearish   Direction = "bearish"
	DirectionMixed     Direction = "mixed"
	DirectionUncertain Direction = "uncertain"
)

// ImpactLevel represents impact magnitude level.
type ImpactLevel string

const (
	ImpactLevelWeak     ImpactLevel = "weak"
	ImpactLevelModerate ImpactLevel = "moderate"
	ImpactLevelStrong   ImpactLevel = "strong"
)

// VerificationVerdict represents causal-chain verification result.
type VerificationVerdict string

const (
	VerificationVerdictPassed  VerificationVerdict = "passed"
	VerificationVerdictWeak    VerificationVerdict = "weak"
	VerificationVerdictInvalid VerificationVerdict = "invalid"
)

// ExposureBucket represents portfolio bucket.
type ExposureBucket string

const (
	ExposureBucketHolding   ExposureBucket = "holding"
	ExposureBucketWatchlist ExposureBucket = "watchlist"
)

// ExposureRelation represents the relation between event entity and portfolio asset.
type ExposureRelation string

const (
	ExposureRelationDirect     ExposureRelation = "direct"
	ExposureRelationProduct    ExposureRelation = "product"
	ExposureRelationCompetitor ExposureRelation = "competitor"
	ExposureRelationSupply     ExposureRelation = "supply"
	ExposureRelationTheme      ExposureRelation = "theme"
	ExposureRelationMacro      ExposureRelation = "macro"
	ExposureRelationUnknown    ExposureRelation = "unknown"
)

// SignalOperator represents monitoring threshold operator.
type SignalOperator string

const (
	SignalOperatorGreaterThan        SignalOperator = "gt"
	SignalOperatorGreaterThanOrEqual SignalOperator = "gte"
	SignalOperatorLessThan           SignalOperator = "lt"
	SignalOperatorLessThanOrEqual    SignalOperator = "lte"
	SignalOperatorEqual              SignalOperator = "eq"
)

// ImpactMapping represents event -> entity -> impact mapping output.
type ImpactMapping struct {
	EventSummary      string                       `json:"event_summary"`
	Items             []ImpactItem                 `json:"items,omitempty"`
	MonitoringSignals []StructuredMonitoringSignal `json:"monitoring_signals,omitempty"`
}

// ImpactItem represents directional impact on a target entity.
type ImpactItem struct {
	EntityName  string      `json:"entity_name"`
	EntityCode  string      `json:"entity_code,omitempty"`
	Direction   Direction   `json:"direction"`
	ImpactScore float64     `json:"impact_score"` // -1.0 ~ +1.0
	ImpactLevel ImpactLevel `json:"impact_level"`
	Confidence  float64     `json:"confidence"`
	Rationale   string      `json:"rationale,omitempty"`
}

// PortfolioExposure represents mapped exposure from an impact item to portfolio assets.
type PortfolioExposure struct {
	AssetName      string           `json:"asset_name"`
	AssetCode      string           `json:"asset_code,omitempty"`
	Bucket         ExposureBucket   `json:"bucket"`
	Relation       ExposureRelation `json:"relation"`
	Direction      Direction        `json:"direction"`
	ImpactScore    float64          `json:"impact_score"`
	ExposureScore  float64          `json:"exposure_score"`
	PositionWeight float64          `json:"position_weight,omitempty"`
	Confidence     float64          `json:"confidence"`
	Rationale      string           `json:"rationale,omitempty"`
}

// CausalVerification represents verifier output.
type CausalVerification struct {
	Verdict    VerificationVerdict `json:"verdict"`
	Score      float64             `json:"score"` // 0.0 ~ 1.0
	Reason     string              `json:"reason,omitempty"`
	Uncertains []string            `json:"uncertains,omitempty"`
}

// StructuredMonitoringSignal represents executable risk-monitoring signal.
type StructuredMonitoringSignal struct {
	Signal    string         `json:"signal"`
	Metric    string         `json:"metric,omitempty"`
	Operator  SignalOperator `json:"operator,omitempty"`
	Threshold string         `json:"threshold,omitempty"`
	Window    string         `json:"window,omitempty"`
	Assets    []string       `json:"assets,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}

// ──────────────────────────────────────────────
// Anchor Pool
// ──────────────────────────────────────────────

// AnchorType represents anchor type
type AnchorType string

const (
	AnchorHoldingDirect     AnchorType = "holding_direct"     // Holding direct (weight 1.0)
	AnchorHoldingProduct    AnchorType = "holding_product"    // Holding product (0.9)
	AnchorHoldingCompetitor AnchorType = "holding_competitor" // Holding competitor (0.75)
	AnchorHoldingSupply     AnchorType = "holding_supply"     // Holding supply chain (0.7)
	AnchorWatchlist         AnchorType = "watchlist"          // Watchlist (0.8)
	AnchorIndustryTheme     AnchorType = "industry_theme"     // Industry theme (0.6)
	AnchorMacroFactor       AnchorType = "macro_factor"       // Macro factor (0.65)
	AnchorGeneralRisk       AnchorType = "general_risk"       // General risk (0.8)
	AnchorSafeHaven         AnchorType = "safe_haven"         // Safe haven signal (0.7)
)

// AnchorWeights represents default weights for anchor types
var AnchorWeights = map[AnchorType]float64{
	AnchorHoldingDirect:     1.0,
	AnchorHoldingProduct:    0.9,
	AnchorHoldingCompetitor: 0.75,
	AnchorHoldingSupply:     0.7,
	AnchorWatchlist:         0.8,
	AnchorIndustryTheme:     0.6,
	AnchorMacroFactor:       0.65,
	AnchorGeneralRisk:       0.8,
	AnchorSafeHaven:         0.7,
}

// Anchor represents anchor definition
type Anchor struct {
	ID           string     `json:"id"`
	Type         AnchorType `json:"type"`
	Text         string     `json:"text"`          // Anchor text
	Embedding    []float64  `json:"embedding"`     // Pre-computed embedding
	Weight       float64    `json:"weight"`        // Weight
	RelatedAsset string     `json:"related_asset"` // Related asset name
	UpdatedAt    time.Time  `json:"updated_at"`
}

// AnchorMatch represents L1 anchor match result
type AnchorMatch struct {
	AnchorID      string     `json:"anchor_id"`
	AnchorText    string     `json:"anchor_text"`
	AnchorType    AnchorType `json:"anchor_type"`
	Similarity    float64    `json:"similarity"`
	WeightedScore float64    `json:"weighted_score"`
	RelatedAsset  string     `json:"related_asset"`
}

// ──────────────────────────────────────────────
// Causal Chain Analysis (L3)
// ──────────────────────────────────────────────

// CausalAnalysis represents L3 deep causal chain analysis
type CausalAnalysis struct {
	EventSummary      string             `json:"event_summary"`
	CausalChains      []CausalChain      `json:"causal_chains"`
	CounterChains     []CausalChain      `json:"counter_chains"` // Counter causal chains (required)
	KeyUncertainties  []string           `json:"key_uncertainties"`
	MonitoringSignals []string           `json:"monitoring_signals"`
	CrossAssetImpacts []CrossAssetImpact `json:"cross_asset_impacts"`
}

// CausalChain represents a causal chain
type CausalChain struct {
	Hops        []CausalHop  `json:"hops"`
	FinalImpact *FinalImpact `json:"final_impact"`
}

// CausalHop represents each hop in causal chain
type CausalHop struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Mechanism  string  `json:"mechanism"`
	Confidence float64 `json:"confidence"`
	Evidence   string  `json:"evidence"`
	TimeLag    string  `json:"time_lag"`
}

// FinalImpact represents final impact of causal chain
type FinalImpact struct {
	Asset     string    `json:"asset"`
	Direction Direction `json:"direction"`
	Magnitude string    `json:"magnitude"` // high / medium / low
	Timeframe string    `json:"timeframe"`
}

// CrossAssetImpact represents cross-asset impact discovery
type CrossAssetImpact struct {
	Asset       string    `json:"asset"`
	AssetCode   string    `json:"asset_code,omitempty"`
	Direction   Direction `json:"direction"`
	Mechanism   string    `json:"mechanism"`
	Confidence  float64   `json:"confidence"`
	IsHolding   bool      `json:"is_holding"`
	SourceEvent string    `json:"source_event"`
}

// ──────────────────────────────────────────────
// Portfolio Context
// ──────────────────────────────────────────────

// PortfolioContext represents portfolio context injected into Pipeline
type PortfolioContext struct {
	Holdings       []Holding   `json:"holdings"`
	Watchlist      []WatchItem `json:"watchlist"`
	IndustryThemes []string    `json:"industry_themes"`
	MacroFactors   []string    `json:"macro_factors"`
}

// Holding represents a holding position
type Holding struct {
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Industry    string   `json:"industry"`
	Weight      float64  `json:"weight"`       // Position weight
	Products    []string `json:"products"`     // Products
	Competitors []string `json:"competitors"`  // Competitors
	SupplyChain []string `json:"supply_chain"` // Supply chain
}

// WatchItem represents a watchlist item
type WatchItem struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// ──────────────────────────────────────────────
// Briefing Structure (Six Modules)
// ──────────────────────────────────────────────

// Briefing represents complete briefing delivered to decision agent
type Briefing struct {
	ID          string      `json:"id"`
	GeneratedAt time.Time   `json:"generated_at"`
	TriggerType TriggerType `json:"trigger_type"`
	Period      string      `json:"period"`

	// Six modules
	MacroOverview   MacroOverview        `json:"macro_overview"`
	PortfolioImpact []PortfolioImpactRow `json:"portfolio_impact"`
	Opportunities   []OpportunityAlert   `json:"opportunities"`
	RiskAlerts      []RiskAlert          `json:"risk_alerts"`
	ConflictSignals []ConflictSignal     `json:"conflict_signals"`
	MonitoringItems []MonitoringItem     `json:"monitoring_items"`

	// Metadata
	TotalNewsProcessed int `json:"total_news_processed"`
	L1Passed           int `json:"l1_passed"`
	L2Passed           int `json:"l2_passed"`
	L3Analyzed         int `json:"l3_analyzed"`
}

// TriggerType represents briefing trigger type
type TriggerType string

const (
	TriggerPreMarket TriggerType = "pre_market" // Before market open (daily 8:30)
	TriggerIntraday  TriggerType = "intraday"   // Intraday scheduled (every 30 min)
	TriggerBreaking  TriggerType = "breaking"   // Breaking event (priority=5 breaking)
	TriggerManual    TriggerType = "manual"     // Manual trigger
)

// MacroOverview represents macro overview module
type MacroOverview struct {
	Summary         string         `json:"summary"`
	MarketSentiment SentimentLevel `json:"market_sentiment"`
	KeyFactors      []MacroFactor  `json:"key_factors"`
	RiskLevel       string         `json:"risk_level"` // low / medium / high / extreme
}

// MacroFactor represents a macro factor
type MacroFactor struct {
	Factor    string    `json:"factor"`
	Direction Direction `json:"direction"`
	Impact    string    `json:"impact"`
}

// PortfolioImpactRow represents portfolio impact matrix row
type PortfolioImpactRow struct {
	Asset         string    `json:"asset"`
	Code          string    `json:"code"`
	RelatedEvents []string  `json:"related_events"`
	NetDirection  Direction `json:"net_direction"`
	Confidence    float64   `json:"confidence"`
	Urgency       string    `json:"urgency"`    // immediate / today / this_week / monitor
	Action        string    `json:"action"`     // Recommended action
	KeyCausal     string    `json:"key_causal"` // Key causal chain (one sentence)
}

// OpportunityAlert represents opportunity discovery
type OpportunityAlert struct {
	Source       string    `json:"source"` // L3/L2/watchlist
	Asset        string    `json:"asset"`
	AssetCode    string    `json:"asset_code,omitempty"`
	Direction    Direction `json:"direction"`
	Mechanism    string    `json:"mechanism"`
	Confidence   float64   `json:"confidence"`
	SourceEvents []string  `json:"source_events"`
}

// RiskAlert represents risk warning
type RiskAlert struct {
	Level       string   `json:"level"` // warning / critical
	Description string   `json:"description"`
	Assets      []string `json:"assets"`
	Events      []string `json:"events"`
	Action      string   `json:"action"`
}

// ConflictSignal represents conflict signal
type ConflictSignal struct {
	Asset        string `json:"asset"`
	BullishEvent string `json:"bullish_event"`
	BearishEvent string `json:"bearish_event"`
	Analysis     string `json:"analysis"`
}

// MonitoringItem represents monitoring item
type MonitoringItem struct {
	Signal    string   `json:"signal"`
	Threshold string   `json:"threshold"`
	Assets    []string `json:"assets"`
	Reason    string   `json:"reason"`
}
