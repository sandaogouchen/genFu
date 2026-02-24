// Package news implements news analysis main pipeline orchestrator.
// Connects Collector → Tagger → Funnel → Briefing Generator four modules,
// supports scheduled/event-driven/manual three trigger modes.
package news

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Pipeline Main Body
// ──────────────────────────────────────────���───

// Pipeline represents news analysis main pipeline
type Pipeline struct {
	collector   *Collector
	tagger      *Tagger
	funnel      *Funnel
	briefingGen *Generator
	store       *PipelineStore
	portfolio   *PortfolioContext
	repo        *Repository // 数据库持久化层

	// Runtime control
	mu           sync.Mutex
	running      bool
	stopCh       chan struct{}
	lastRunTime  time.Time

	// Configuration
	config PipelineConfig
}

// PipelineConfig represents pipeline configuration
type PipelineConfig struct {
	// Scheduled trigger configuration
	PreMarketTime    string        `json:"pre_market_time"`    // Before market open time (e.g., "08:30")
	IntradayInterval time.Duration `json:"intraday_interval"`  // Intraday polling interval (e.g., 30min)
	TradingStart     string        `json:"trading_start"`      // Trading start time
	TradingEnd       string        `json:"trading_end"`        // Trading end time

	// News lookback window
	LookbackDuration time.Duration `json:"lookback_duration"` // News lookback time window
}

// DefaultPipelineConfig returns default configuration
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		PreMarketTime:    "08:30",
		IntradayInterval: 30 * time.Minute,
		TradingStart:     "09:30",
		TradingEnd:       "15:00",
		LookbackDuration: 24 * time.Hour,
	}
}

// NewPipeline creates a pipeline
func NewPipeline(
	col *Collector,
	tag *Tagger,
	fnl *Funnel,
	portfolio *PortfolioContext,
	repo *Repository, // 数据库持久化层
	opts ...PipelineOption,
) *Pipeline {
	store := NewPipelineStore()
	p := &Pipeline{
		collector:   col,
		tagger:      tag,
		funnel:      fnl,
		briefingGen: NewGenerator(portfolio),
		store:       store,
		portfolio:   portfolio,
		repo:        repo,
		config:      DefaultPipelineConfig(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type PipelineOption func(*Pipeline)

func WithPipelineConfig(c PipelineConfig) PipelineOption {
	return func(p *Pipeline) { p.config = c }
}

// GetStore returns the pipeline store
func (p *Pipeline) GetStore() *PipelineStore {
	return p.store
}

// ──────────────────────────────────────────────
// Core Execution Logic
// ──────────────────────────────────────────────

// RunResult represents single run result
type RunResult struct {
	Briefing       *Briefing `json:"briefing"`
	TotalCollected int       `json:"total_collected"`
	TotalTagged    int       `json:"total_tagged"`
	L1Passed       int       `json:"l1_passed"`
	L2Passed       int       `json:"l2_passed"`
	L3Analyzed     int       `json:"l3_analyzed"`
	Duration       time.Duration `json:"duration"`
	Error          error     `json:"error,omitempty"`
}

// Run executes a complete news analysis pipeline once
func (p *Pipeline) Run(ctx context.Context, triggerType TriggerType) (*RunResult, error) {
	startTime := time.Now()
	result := &RunResult{}

	log.Printf("[Pipeline] 开始执行, 触发类型: %s", triggerType)

	// ── Step 1: News Collection ──
	since := time.Now().Add(-p.config.LookbackDuration)
	rawNews, err := p.collector.Collect(ctx, since)
	if err != nil {
		result.Error = fmt.Errorf("collection failed: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}
	result.TotalCollected = len(rawNews)
	log.Printf("[Pipeline] 采集完成: %d 条新闻", len(rawNews))

	if len(rawNews) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// ── Step 2: Classification and Tagging ──
	events, err := p.tagger.TagBatch(ctx, rawNews)
	if err != nil {
		result.Error = fmt.Errorf("tagging failed: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}
	result.TotalTagged = len(events)
	log.Printf("[Pipeline] 打标完成: %d 条事件", len(events))

	// ── Step 3: Three-Layer Funnel Filtering ──
	filtered, err := p.funnel.Filter(ctx, events)
	if err != nil {
		log.Printf("[Pipeline] 漏斗筛选部分失败: %v", err)
	}

	// Count each layer passed
	l1Count, l2Count, l3Count := 0, 0, 0
	for _, e := range events {
		if e.FunnelResult != nil {
			if e.FunnelResult.L1Pass {
				l1Count++
			}
			if e.FunnelResult.L2Pass {
				l2Count++
			}
			if e.FunnelResult.L3Analysis != nil {
				l3Count++
			}
		}
	}
	result.L1Passed = l1Count
	result.L2Passed = l2Count
	result.L3Analyzed = l3Count
	log.Printf("[Pipeline] 漏斗筛选: L1=%d, L2=%d, L3=%d", l1Count, l2Count, l3Count)

	// ── Step 4: Generate Briefing ──
	brief := p.briefingGen.Generate(filtered, triggerType, result.TotalCollected, l1Count, l2Count)
	result.Briefing = brief

	// Save to Store (内存)
	p.store.Save(brief)
	p.store.SaveEvents(filtered)

	// 持久化到数据库
	if p.repo != nil {
		// 保存所有事件到数据库
		for _, event := range filtered {
			if _, err := p.repo.CreateEvent(ctx, *event); err != nil {
				log.Printf("[Pipeline] 保存事件失败: %v, title=%s", err, event.Title)
			}
		}
		// 保存简报到数据库
		if err := p.repo.SaveBriefing(ctx, *brief); err != nil {
			log.Printf("[Pipeline] 保存简报失败: %v", err)
		} else {
			log.Printf("[Pipeline] 已保存 %d 个事件和 1 个简报到数据库", len(filtered))
		}
	} else {
		log.Printf("[Pipeline] 警告: 未配置 Repository，数据仅保存在内存中")
	}

	result.Duration = time.Since(startTime)
	p.lastRunTime = time.Now()

	log.Printf("[Pipeline] 完成, 耗时: %v, 简报ID: %s", result.Duration, brief.ID)

	return result, nil
}

// ──────────────────────────────────────────────
// Scheduled Dispatching
// ──────────────────────────────────────────────

// Start starts scheduled dispatching
func (p *Pipeline) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("pipeline already running")
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.mu.Unlock()

	go p.schedulerLoop(ctx)

	log.Printf("[Pipeline] 调度器已启动")
	return nil
}

// Stop stops scheduled dispatching
func (p *Pipeline) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		close(p.stopCh)
		p.running = false
		log.Printf("[Pipeline] 调度器已停止")
	}
}

func (p *Pipeline) schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case now := <-ticker.C:
			p.checkAndTrigger(ctx, now)
		}
	}
}

func (p *Pipeline) checkAndTrigger(ctx context.Context, now time.Time) {
	nowStr := now.Format("15:04")

	// Pre-market trigger
	if nowStr == p.config.PreMarketTime {
		log.Printf("[Scheduler] 触发开盘前分析")
		go func() {
			if _, err := p.Run(ctx, TriggerPreMarket); err != nil {
				log.Printf("[Scheduler] 开盘前分析失败: %v", err)
			}
		}()
		return
	}

	// Intraday scheduled trigger
	if isInTradingHours(nowStr, p.config.TradingStart, p.config.TradingEnd) {
		if time.Since(p.lastRunTime) >= p.config.IntradayInterval {
			log.Printf("[Scheduler] 触发盘中增量分析")
			go func() {
				if _, err := p.Run(ctx, TriggerIntraday); err != nil {
					log.Printf("[Scheduler] 盘中分析失败: %v", err)
				}
			}()
		}
	}
}

// ──────────────────────────────────────────────
// Breaking News Fast Path
// ──────��───────────────────────────────────────

// HandleBreakingNews handles breaking news (skips L1/L2, goes directly to deep analysis)
func (p *Pipeline) HandleBreakingNews(ctx context.Context, news RawNews) (*RunResult, error) {
	startTime := time.Now()
	log.Printf("[Pipeline] 突发事件快速通道: %s", news.Title)

	// Tag directly
	event, err := p.tagger.Tag(ctx, news)
	if err != nil {
		return nil, fmt.Errorf("tagging breaking news: %w", err)
	}

	// Mark as high priority
	event.Labels.Novelty = NoveltyBreaking
	event.Labels.Predictability = PredictabilityUnscheduled

	// Build temporary FunnelResult
	event.FunnelResult = &FunnelResult{
		L1Pass:     true,
		L1Score:    1.0,
		L2Pass:     true,
		L2Relevance: RelevanceHigh,
		L2Priority: 5,
		L2NeedsDeep: true,
	}

	// Go directly to L3 deep analysis
	events := []*NewsEvent{event}
	filtered, _ := p.funnel.Filter(ctx, events)
	if len(filtered) == 0 {
		filtered = events
	}

	// Generate briefing
	brief := p.briefingGen.Generate(filtered, TriggerBreaking, 1, 1, 1)
	p.store.Save(brief)
	p.store.SaveEvents(filtered)

	return &RunResult{
		Briefing:       brief,
		TotalCollected: 1,
		TotalTagged:    1,
		L1Passed:       1,
		L2Passed:       1,
		L3Analyzed:     1,
		Duration:       time.Since(startTime),
	}, nil
}

// ──────────────────────────────────────────────
// Helper Functions
// ──────────────────────────────────────────────

func isInTradingHours(now, start, end string) bool {
	return now >= start && now <= end
}

// ──────────────────────────────────────────────
// Pipeline Store (in-memory implementation)
// ──────────────────────────────────────────────

// PipelineStore represents in-memory pipeline store
type PipelineStore struct {
	mu           sync.RWMutex
	latestBrief  *Briefing
	events       []*NewsEvent
}

// NewPipelineStore creates a new pipeline store
func NewPipelineStore() *PipelineStore {
	return &PipelineStore{}
}

// Save saves a briefing
func (s *PipelineStore) Save(brief *Briefing) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latestBrief = brief
}

// SaveEvents saves events
func (s *PipelineStore) SaveEvents(events []*NewsEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = events
}

// GetLatestBriefing returns the latest briefing
func (s *PipelineStore) GetLatestBriefing() *Briefing {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestBrief
}

// GetEvents returns all events
func (s *PipelineStore) GetEvents() []*NewsEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events
}
