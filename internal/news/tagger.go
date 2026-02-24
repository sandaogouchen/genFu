// Package news implements two-level classification and labeling engine.
// Level 1: Keyword rule matching (fast, zero-cost, high certainty, covers ~25 high-frequency event types)
// Level 2: Embedding/LLM classification (covers long-tail event types)
package news

import (
	"context"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// Interface Definitions
// ──────────────────────────────────────────────

// EmbeddingClassifier interface for embedding-based classification
type EmbeddingClassifier interface {
	Classify(ctx context.Context, title, summary string) ([]EventDomain, []string, float64, error)
}

// LLMLabelService interface for LLM-based labeling (for multi-dimensional labels)
type LLMLabelService interface {
	Label(ctx context.Context, title, summary string, domains []EventDomain) (*LabelSet, error)
}

// ──────────────────────────────────────────────
// Keyword Rules
// ──────────────────────────────────────────────

// KeywordRule represents a keyword classification rule
type KeywordRule struct {
	RuleID      string       `json:"rule_id"`
	Keywords    []string     `json:"keywords"`
	Domain      EventDomain  `json:"domain"`
	EventType   string       `json:"event_type"`
	DefaultConf float64      `json:"default_confidence"` // Rule match gives 0.8
}

// defaultKeywordRules returns default keyword rule library (~25 high-frequency event types)
func defaultKeywordRules() []KeywordRule {
	return []KeywordRule{
		// ─── Macroeconomic Domain ───
		{
			RuleID:      "macro.central_bank_rate",
			Keywords:    []string{"加息", "降息", "利率决议", "美联储", "FOMC", "LPR", "MLF", "央行", "基准利率", "联邦基金利率", "interest rate", "fed rate"},
			Domain:      DomainMacro,
			EventType:   "央行利率决议",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "macro.inflation",
			Keywords:    []string{"CPI", "PPI", "通胀", "通缩", "物价", "消费者价格", "生产者价格", "inflation", "deflation"},
			Domain:      DomainMacro,
			EventType:   "通胀数据",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "macro.gdp",
			Keywords:    []string{"GDP", "经济增长", "经济衰退", "经济放缓", "负增长", "recession", "economic growth"},
			Domain:      DomainMacro,
			EventType:   "GDP数据",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "macro.employment",
			Keywords:    []string{"非农", "失业率", "劳动力", "就业", "新增就业", "nonfarm", "unemployment", "jobless"},
			Domain:      DomainMacro,
			EventType:   "就业数据",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "macro.pmi",
			Keywords:    []string{"PMI", "制造业PMI", "服务业PMI", "采购经理人指数"},
			Domain:      DomainMacro,
			EventType:   "PMI数据",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "macro.fx",
			Keywords:    []string{"人民币汇率", "美元指数", "汇率", "外汇储备", "汇率政策", "贬值", "升值", "USDCNY"},
			Domain:      DomainMacro,
			EventType:   "汇率政策",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "macro.liquidity",
			Keywords:    []string{"逆回购", "MLF操作", "准备金率", "降准", "公开市场操作", "流动性", "Shibor"},
			Domain:      DomainMacro,
			EventType:   "流动性事件",
			DefaultConf: 0.8,
		},

		// ─── Geopolitical Domain ───
		{
			RuleID:      "geo.military",
			Keywords:    []string{"军事打击", "空袭", "导弹", "战争", "冲突升级", "停火", "军事冲突", "武装", "轰炸", "missile", "airstrike", "war"},
			Domain:      DomainGeopolitical,
			EventType:   "军事冲突",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "geo.trade",
			Keywords:    []string{"关税", "贸易战", "反倾销", "贸易摩擦", "贸易制裁", "加征关税", "tariff", "trade war"},
			Domain:      DomainGeopolitical,
			EventType:   "贸易摩擦",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "geo.sanction",
			Keywords:    []string{"实体清单", "出口管制", "技术封锁", "制裁", "禁令", "sanctions", "entity list", "export control"},
			Domain:      DomainGeopolitical,
			EventType:   "制裁",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "geo.diplomacy",
			Keywords:    []string{"峰会", "国事访问", "断交", "外交", "建交", "首脑会谈", "summit", "diplomatic"},
			Domain:      DomainGeopolitical,
			EventType:   "外交事件",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "geo.energy_security",
			Keywords:    []string{"海峡封锁", "管道", "OPEC", "减产协议", "石油禁运", "能源安全", "能源危机"},
			Domain:      DomainGeopolitical,
			EventType:   "能源安全",
			DefaultConf: 0.8,
		},

		// ─── Industry Domain ───
		{
			RuleID:      "ind.tech_breakthrough",
			Keywords:    []string{"发布", "开源", "突破", "新一代", "重大进展", "里程碑", "launch", "breakthrough", "open source"},
			Domain:      DomainIndustry,
			EventType:   "技术突破",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "ind.supply_chain",
			Keywords:    []string{"断供", "缺货", "交付延迟", "供应链", "供应中断", "supply chain", "shortage"},
			Domain:      DomainIndustry,
			EventType:   "供应链变动",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "ind.capacity",
			Keywords:    []string{"扩产", "减产", "停产", "检修", "产能", "产线", "capacity"},
			Domain:      DomainIndustry,
			EventType:   "产能变化",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "ind.price",
			Keywords:    []string{"涨价", "降价", "价格战", "提价", "调价", "价格上调", "价格下调"},
			Domain:      DomainIndustry,
			EventType:   "行业价格变动",
			DefaultConf: 0.8,
		},

		// ─── Corporate Domain ───
		{
			RuleID:      "corp.earnings",
			Keywords:    []string{"财报", "季报", "年报", "净利润", "营收", "EPS", "业绩", "业绩预告", "盈利", "亏损", "earnings", "revenue"},
			Domain:      DomainCorporate,
			EventType:   "财报",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "corp.ma",
			Keywords:    []string{"收购", "合并", "要约", "私有化", "并购", "merger", "acquisition", "takeover"},
			Domain:      DomainCorporate,
			EventType:   "并购活动",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "corp.product",
			Keywords:    []string{"新产品", "上线", "商用", "发布会", "产品发布", "正式推出"},
			Domain:      DomainCorporate,
			EventType:   "产品发布",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "corp.management",
			Keywords:    []string{"CEO", "CFO", "CTO", "离职", "任命", "换帅", "管理层", "董事长", "总裁", "resignation", "appointed"},
			Domain:      DomainCorporate,
			EventType:   "管理层变动",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "corp.shareholder",
			Keywords:    []string{"增持", "减��", "回购", "质押", "大宗交易", "举牌", "buyback"},
			Domain:      DomainCorporate,
			EventType:   "股东行为",
			DefaultConf: 0.8,
		},

		// ─── Regulatory Domain ───
		{
			RuleID:      "reg.policy",
			Keywords:    []string{"政策发布", "新规", "监管", "征求意见", "管理办法", "实施细则", "regulation"},
			Domain:      DomainRegulatory,
			EventType:   "政策发布",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "reg.investigation",
			Keywords:    []string{"调查", "处罚", "罚款", "约谈", "整改", "反垄断", "antitrust", "investigation"},
			Domain:      DomainRegulatory,
			EventType:   "调查处罚",
			DefaultConf: 0.8,
		},

		// ─── Market Domain ───
		{
			RuleID:      "mkt.fund_flow",
			Keywords:    []string{"北向资金", "南向资金", "外资流入", "ETF申赎", "融资融券", "资金流向", "外资流出"},
			Domain:      DomainMarket,
			EventType:   "资金流向",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "mkt.analyst",
			Keywords:    []string{"评级", "上调", "下调", "目标价", "买入评级", "卖出评级", "upgrade", "downgrade", "target price"},
			Domain:      DomainMarket,
			EventType:   "分析师评级",
			DefaultConf: 0.8,
		},

		// ─── Technology Domain ───
		{
			RuleID:      "tech.ai",
			Keywords:    []string{"大模型", "GPT", "LLM", "AI模型", "人工智能", "深度学习", "AGI", "生成式AI"},
			Domain:      DomainTechnology,
			EventType:   "AI模型",
			DefaultConf: 0.8,
		},
		{
			RuleID:      "tech.chip",
			Keywords:    []string{"芯片", "半导体", "晶圆", "光刻机", "制程", "先进封装", "semiconductor", "chip"},
			Domain:      DomainTechnology,
			EventType:   "芯片",
			DefaultConf: 0.8,
		},
	}
}

// ──────────────────────────────────────────────
// Tagger Classification Engine
// ──────────────────────────────────────────────

// Tagger represents two-level classification tagger
type Tagger struct {
	keywordRules    []KeywordRule
	embedClassifier EmbeddingClassifier // optional
	llmLabelSvc     LLMLabelService     // optional
}

// NewTagger creates a classification tagger
func NewTagger(opts ...TaggerOption) *Tagger {
	t := &Tagger{
		keywordRules: defaultKeywordRules(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// TaggerOption represents configuration option
type TaggerOption func(*Tagger)

func WithEmbeddingClassifier(c EmbeddingClassifier) TaggerOption {
	return func(t *Tagger) { t.embedClassifier = c }
}

func WithLLMLabelService(s LLMLabelService) TaggerOption {
	return func(t *Tagger) { t.llmLabelSvc = s }
}

func WithCustomRules(rules []KeywordRule) TaggerOption {
	return func(t *Tagger) { t.keywordRules = append(t.keywordRules, rules...) }
}

// TagResult represents tagging result
type TagResult struct {
	Domains        []EventDomain
	EventTypes     []string
	Confidence     float64
	Method         string // "keyword" / "embedding" / "keyword+embedding"
	MatchedRuleIDs []string
}

// ──────────────────────────────────────────────
// Level 1: Keyword Rule Matching
// ──────────────────────────────────────────────

// matchKeywordRules performs keyword rule matching
func (t *Tagger) matchKeywordRules(title, summary string) TagResult {
	text := strings.ToLower(title + " " + summary)
	domainSet := make(map[EventDomain]bool)
	typeSet := make(map[string]bool)
	var ruleIDs []string
	maxConf := 0.0

	for _, rule := range t.keywordRules {
		matched := false
		matchCount := 0
		for _, kw := range rule.Keywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				matched = true
				matchCount++
			}
		}
		if matched {
			domainSet[rule.Domain] = true
			typeSet[rule.EventType] = true
			ruleIDs = append(ruleIDs, rule.RuleID)
			// Multiple keyword hits boost confidence
			conf := rule.DefaultConf
			if matchCount >= 3 {
				conf = min64(conf+0.1, 0.95)
			}
			if conf > maxConf {
				maxConf = conf
			}
		}
	}

	if len(domainSet) == 0 {
		return TagResult{Method: "keyword"}
	}

	domains := make([]EventDomain, 0, len(domainSet))
	for d := range domainSet {
		domains = append(domains, d)
	}
	types := make([]string, 0, len(typeSet))
	for typ := range typeSet {
		types = append(types, typ)
	}

	return TagResult{
		Domains:        domains,
		EventTypes:     types,
		Confidence:     maxConf,
		Method:         "keyword",
		MatchedRuleIDs: ruleIDs,
	}
}

// ──────────────────────────────────────────────
// Complete Tagging Flow
// ──────────────────────────────────────────────

// Tag performs complete two-level classification and tagging on a single news item
func (t *Tagger) Tag(ctx context.Context, news RawNews) (*NewsEvent, error) {
	// Level 1: Keyword rule matching
	kwResult := t.matchKeywordRules(news.Title, news.Summary)

	// Level 2: Embedding classification supplement (when keyword rules match insufficiently)
	finalDomains := kwResult.Domains
	finalTypes := kwResult.EventTypes
	finalConf := kwResult.Confidence
	method := kwResult.Method

	if t.embedClassifier != nil && (len(kwResult.Domains) == 0 || kwResult.Confidence < 0.6) {
		embDomains, embTypes, embConf, err := t.embedClassifier.Classify(ctx, news.Title, news.Summary)
		if err == nil && len(embDomains) > 0 {
			if len(kwResult.Domains) == 0 {
				// Keyword no match, use Embedding result
				finalDomains = embDomains
				finalTypes = embTypes
				finalConf = embConf
				method = "embedding"
			} else {
				// Merge results
				domainMap := make(map[EventDomain]bool)
				for _, d := range kwResult.Domains {
					domainMap[d] = true
				}
				for _, d := range embDomains {
					domainMap[d] = true
				}
				finalDomains = make([]EventDomain, 0, len(domainMap))
				for d := range domainMap {
					finalDomains = append(finalDomains, d)
				}
				typeMap := make(map[string]bool)
				for _, typ := range kwResult.EventTypes {
					typeMap[typ] = true
				}
				for _, typ := range embTypes {
					typeMap[typ] = true
				}
				finalTypes = make([]string, 0, len(typeMap))
				for typ := range typeMap {
					finalTypes = append(finalTypes, typ)
				}
				finalConf = max64(kwResult.Confidence, embConf)
				method = "keyword+embedding"
			}
		}
	}

	// Build multi-dimensional labels
	labels := LabelSet{
		Sentiment:      0,    // Default neutral
		Novelty:        NoveltyBreaking,
		Predictability: PredictabilityUnscheduled,
		Timeframe:      TimeframeShort,
	}

	// Use LLM for fine-grained labels (if available)
	if t.llmLabelSvc != nil && len(finalDomains) > 0 {
		llmLabels, err := t.llmLabelSvc.Label(ctx, news.Title, news.Summary, finalDomains)
		if err == nil && llmLabels != nil {
			labels = *llmLabels
		}
	}

	event := &NewsEvent{
		ID:                 news.ID,
		Title:              news.Title,
		Summary:            news.Summary,
		Content:            news.Content,
		Source:             news.Source,
		SourceType:         news.SourceType,
		URL:                news.URL,
		PublishedAt:        news.PublishedAt,
		ProcessedAt:        time.Now(),
		Domains:            finalDomains,
		EventTypes:         finalTypes,
		Labels:             labels,
		ClassifyConfidence: finalConf,
		ClassifyMethod:     method,
		DedupClusterID:     news.TitleHash,
		RelatedSources:     news.RelatedSources,
	}

	return event, nil
}

// TagBatch performs batch tagging
func (t *Tagger) TagBatch(ctx context.Context, newsList []RawNews) ([]*NewsEvent, error) {
	events := make([]*NewsEvent, 0, len(newsList))
	for _, news := range newsList {
		event, err := t.Tag(ctx, news)
		if err != nil {
			continue // Single failure doesn't affect overall
		}
		events = append(events, event)
	}
	return events, nil
}

// ──────────────────────────────────────────────
// Helper Functions
// ──────────────────────────────────────────────

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
