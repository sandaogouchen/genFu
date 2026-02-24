// Package news implements news collection and preprocessing.
// Includes deduplication (exact MD5 + semantic Embedding) and noise filtering (ads/quote broadcasts/short content).
package news

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"genFu/internal/rsshub"
)

// ──────────────────────────────────────────────
// Interface Definitions
// ──────────────────────────────────────────────

// EmbeddingService interface for text vectorization
type EmbeddingService interface {
	Encode(ctx context.Context, texts []string) ([][]float64, error)
}

// NewsSource interface for different data sources
type NewsSource interface {
	Name() string
	Type() SourceType
	Fetch(ctx context.Context, since time.Time) ([]RawNews, error)
}

// RSSHubSource adapts RSSHub client to NewsSource interface
type RSSHubSource struct {
	client *rsshub.Client
	routes []string
	limit  int
}

func NewRSSHubSource(client *rsshub.Client, routes []string, limit int) *RSSHubSource {
	return &RSSHubSource{
		client: client,
		routes: routes,
		limit:  limit,
	}
}

func (s *RSSHubSource) Name() string {
	return "rsshub"
}

func (s *RSSHubSource) Type() SourceType {
	return SourceFinancialMedia
}

func (s *RSSHubSource) Fetch(ctx context.Context, since time.Time) ([]RawNews, error) {
	if s.client == nil || len(s.routes) == 0 {
		log.Printf("[RSSHubSource] 客户端未初始化或路由为空")
		return nil, nil
	}

	var allNews []RawNews
	for _, route := range s.routes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}

		log.Printf("[RSSHubSource] 正在抓取路由: %s", route)
		items, err := s.client.Fetch(route, s.limit)
		if err != nil {
			log.Printf("[RSSHubSource] 抓取失败 route=%s error=%v", route, err)
			continue
		}

		log.Printf("[RSSHubSource] 抓取成功 route=%s count=%d", route, len(items))

		for _, item := range items {
			if item.Title == "" || item.Link == "" {
				continue
			}

			// Check published time
			if item.PublishedAt != nil && item.PublishedAt.Before(since) {
				log.Printf("[RSSHubSource] 新闻被时间过滤 title=%s published=%v", item.Title, item.PublishedAt)
				continue
			}

			news := RawNews{
				ID:          computeMD5(item.Title + "|" + route),
				Title:       item.Title,
				Summary:     item.Description,
				Content:     item.Description,
				Source:      route,
				SourceType:  SourceFinancialMedia,
				URL:         item.Link,
				PublishedAt: time.Now(),
				CollectedAt: time.Now(),
				Language:    "zh",
			}
			if item.PublishedAt != nil {
				news.PublishedAt = *item.PublishedAt
			}
			allNews = append(allNews, news)
		}
	}

	log.Printf("[RSSHubSource] 总计采集 %d 条新闻", len(allNews))
	return allNews, nil
}

// ──────────────────────────────────────────────
// Collector Main
// ──────────────────────────────────────────────

// Collector collects news from multiple sources
type Collector struct {
	sources    []NewsSource
	embedSvc   EmbeddingService
	dedupStore *DedupStore
	noiseRules []NoiseRule
	mu         sync.Mutex
}

// NewCollector creates a new news collector
func NewCollector(embedSvc EmbeddingService, sources ...NewsSource) *Collector {
	// 显式处理: 如果传入 nil 具体值，保持接口为 nil
	var svc EmbeddingService
	if embedSvc != nil {
		svc = embedSvc
	}
	log.Printf("[Collector] 初始化 embedSvc=%v nil=%v", svc, svc == nil)
	return &Collector{
		sources:    sources,
		embedSvc:   svc,
		dedupStore: NewDedupStore(),
		noiseRules: defaultNoiseRules(),
	}
}

// Collect collects news from all sources and performs deduplication and noise filtering
func (c *Collector) Collect(ctx context.Context, since time.Time) ([]RawNews, error) {
	var (
		allNews []RawNews
		mu      sync.Mutex
		wg      sync.WaitGroup
		errs    []error
	)

	// Concurrent collection
	for _, src := range c.sources {
		wg.Add(1)
		go func(s NewsSource) {
			defer wg.Done()
			news, err := s.Fetch(ctx, since)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("source %s: %w", s.Name(), err))
				return
			}
			allNews = append(allNews, news...)
		}(src)
	}
	wg.Wait()

	if len(allNews) == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("all sources failed: %v", errs)
		}
		return nil, nil
	}

	// Step 1: Noise filtering
	filtered := c.filterNoise(allNews)

	// Step 2: Exact deduplication (title + source MD5 hash)
	exactDeduped := c.exactDedup(filtered)

	// Step 3: Semantic deduplication (title + summary Embedding > 0.85)
	semanticDeduped, err := c.semanticDedup(ctx, exactDeduped)
	if err != nil {
		// Fallback: if semantic dedup fails, use exact dedup result
		return exactDeduped, nil
	}

	return semanticDeduped, nil
}

// ──────────────────────────────────────────────
// Noise Filtering Rules Engine
// ──────────────────────────────────────────────

// NoiseRule represents a noise filtering rule
type NoiseRule struct {
	Name      string
	Reason    string
	MatchFunc func(news RawNews) bool
}

// defaultNoiseRules returns default noise filtering rule set
func defaultNoiseRules() []NoiseRule {
	return []NoiseRule{
		{
			Name:   "ad_filter",
			Reason: "No information value: ads/promotion/sponsorship",
			MatchFunc: func(n RawNews) bool {
				keywords := []string{"广告", "推广", "赞助", "sponsored", "advertisement", "promoted"}
				title := strings.ToLower(n.Title)
				for _, kw := range keywords {
					if strings.Contains(title, kw) {
						return true
					}
				}
				return false
			},
		},
		{
			Name:   "quote_broadcast_filter",
			Reason: "Already reflected in market data: pure quote broadcast",
			MatchFunc: func(n RawNews) bool {
				// Pure "close up/down X%" without reason analysis
				quoteKeywords := []string{"收涨", "收跌", "收盘", "报收"}
				analysisKeywords := []string{"因为", "由于", "受", "影响", "分析", "预计"}

				title := n.Title
				hasQuote := false
				for _, kw := range quoteKeywords {
					if strings.Contains(title, kw) {
						hasQuote = true
						break
					}
				}
				if !hasQuote {
					return false
				}
				// Check if there's reason analysis
				text := title + n.Summary
				for _, kw := range analysisKeywords {
					if strings.Contains(text, kw) {
						return false // Has analysis, keep it
					}
				}
				return true
			},
		},
		{
			Name:   "short_empty_filter",
			Reason: "Insufficient information: summary too short",
			MatchFunc: func(n RawNews) bool {
				// Summary < 20 chars and title < 10 chars
				return len([]rune(n.Summary)) < 20 && len([]rune(n.Title)) < 10
			},
		},
	}
}

// filterNoise applies noise filtering rules
func (c *Collector) filterNoise(news []RawNews) []RawNews {
	result := make([]RawNews, 0, len(news))
	for _, n := range news {
		noisy := false
		for _, rule := range c.noiseRules {
			if rule.MatchFunc(n) {
				noisy = true
				break
			}
		}
		if !noisy {
			result = append(result, n)
		}
	}
	return result
}

// ──────────────────────────────────────────────
// Two-Level Deduplication
// ──────────────────────────────────────────────

// exactDedup performs exact deduplication using title + source MD5 hash
func (c *Collector) exactDedup(news []RawNews) []RawNews {
	result := make([]RawNews, 0, len(news))
	for _, n := range news {
		hash := computeMD5(n.Title + "|" + n.Source)
		n.TitleHash = hash
		if c.dedupStore.TryAddHash(hash) {
			result = append(result, n)
		}
	}
	return result
}

// semanticDedup performs semantic deduplication using title + summary Embedding similarity > 0.85
// Merges by keeping the one with most information (using full text length as proxy)
func (c *Collector) semanticDedup(ctx context.Context, news []RawNews) ([]RawNews, error) {
	// 使用反射检查接口内部是否为 nil
	isNil := c.embedSvc == nil
	if !isNil {
		v := reflect.ValueOf(c.embedSvc)
		if v.Kind() == reflect.Ptr && v.IsNil() {
			isNil = true
		}
	}

	log.Printf("[Collector.semanticDedup] 检查 embedSvc nil=%v len(news)=%d", isNil, len(news))
	if len(news) == 0 || isNil {
		log.Printf("[Collector.semanticDedup] 跳过语义去重")
		return news, nil
	}

	// Build texts to encode
	texts := make([]string, len(news))
	for i, n := range news {
		summary := n.Summary
		if len([]rune(summary)) > 150 {
			summary = string([]rune(summary)[:150])
		}
		texts[i] = n.Title + "。" + summary
	}

	log.Printf("[Collector.semanticDedup] 开始调用 Embedding API, 文本数量=%d", len(texts))
	// Batch encode
	embeddings, err := c.embedSvc.Encode(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("encoding failed: %w", err)
	}

	// Greedy clustering deduplication
	const similarityThreshold = 0.85
	type cluster struct {
		representativeIdx int
		memberIndices     []int
	}

	var clusters []cluster
	assigned := make([]bool, len(news))

	for i := 0; i < len(news); i++ {
		if assigned[i] {
			continue
		}
		cl := cluster{representativeIdx: i, memberIndices: []int{i}}
		assigned[i] = true

		for j := i + 1; j < len(news); j++ {
			if assigned[j] {
				continue
			}
			sim := cosineSimilarity(embeddings[i], embeddings[j])
			if sim > similarityThreshold {
				cl.memberIndices = append(cl.memberIndices, j)
				assigned[j] = true
			}
		}

		// Select the one with most information (longest full text) as representative
		bestIdx := cl.representativeIdx
		bestLen := len(news[bestIdx].Content)
		for _, idx := range cl.memberIndices {
			if len(news[idx].Content) > bestLen {
				bestIdx = idx
				bestLen = len(news[idx].Content)
			}
		}
		cl.representativeIdx = bestIdx
		clusters = append(clusters, cl)
	}

	// Output deduplicated result
	result := make([]RawNews, 0, len(clusters))
	for _, cl := range clusters {
		rep := news[cl.representativeIdx]
		// Record other sources for cross-validation
		for _, idx := range cl.memberIndices {
			if idx != cl.representativeIdx {
				rep.RelatedSources = append(rep.RelatedSources, news[idx].Source+" | "+news[idx].URL)
			}
		}
		result = append(result, rep)
	}

	return result, nil
}

// ──────────────────────────────────────────────
// Deduplication Store
// ──────────────────────────────────────────────

// DedupStore represents deduplication hash store (in-memory implementation, can be replaced with Redis in production)
type DedupStore struct {
	mu     sync.RWMutex
	hashes map[string]bool
}

// NewDedupStore creates a new dedup store
func NewDedupStore() *DedupStore {
	return &DedupStore{hashes: make(map[string]bool)}
}

// TryAddHash tries to add a hash, returns true if it's new
func (s *DedupStore) TryAddHash(hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hashes[hash] {
		return false
	}
	s.hashes[hash] = true
	return true
}

// Reset clears the dedup store
func (s *DedupStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hashes = make(map[string]bool)
}

// ──────────────────────────────────────────────
// Utility Functions
// ──────────────────────────────────────────────

func computeMD5(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func cosineSimilarity(a, b []float64) float64 {
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
