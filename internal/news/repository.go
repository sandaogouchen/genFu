package news

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"genFu/internal/db"
)

type Repository struct {
	db *db.DB
}

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateItem(ctx context.Context, item NewsItem) (NewsItem, bool, error) {
	if r == nil || r.db == nil {
		return NewsItem{}, false, errors.New("repository_not_initialized")
	}
	if strings.TrimSpace(item.Source) == "" || strings.TrimSpace(item.GUID) == "" {
		return NewsItem{}, false, errors.New("invalid_item_key")
	}
	row := r.db.QueryRowContext(ctx, `
		insert into news_items(source, title, link, guid, published_at, content)
		values (?, ?, ?, ?, ?, ?)
		on conflict(source, guid) do update
		set title = excluded.title,
			link = excluded.link,
			published_at = excluded.published_at,
			content = excluded.content
		returning id, source, title, link, guid, published_at, content, fetched_at
	`, item.Source, item.Title, item.Link, item.GUID, item.PublishedAt, item.Content)
	var created NewsItem
	var publishedRaw sql.NullString
	var fetchedRaw sql.NullString
	if err := row.Scan(&created.ID, &created.Source, &created.Title, &created.Link, &created.GUID, &publishedRaw, &created.Content, &fetchedRaw); err != nil {
		return NewsItem{}, false, err
	}
	if parsed, ok := db.ParseTime(publishedRaw); ok {
		created.PublishedAt = &parsed
	}
	if parsed, ok := db.ParseTime(fetchedRaw); ok {
		created.FetchedAt = parsed
	}
	return created, true, nil
}

func (r *Repository) CreateBrief(ctx context.Context, itemID int64, sentiment string, brief string, keywords []string) (NewsBrief, error) {
	if r == nil || r.db == nil {
		return NewsBrief{}, errors.New("repository_not_initialized")
	}
	if itemID == 0 {
		return NewsBrief{}, errors.New("invalid_item_id")
	}
	if strings.TrimSpace(sentiment) == "" || strings.TrimSpace(brief) == "" {
		return NewsBrief{}, errors.New("invalid_brief")
	}
	payload, err := json.Marshal(keywords)
	if err != nil {
		return NewsBrief{}, err
	}
	row := r.db.QueryRowContext(ctx, `
		insert into news_briefs(item_id, sentiment, brief, keywords)
		values (?, ?, ?, ?)
		returning id, item_id, sentiment, brief, keywords, created_at
	`, itemID, sentiment, brief, payload)
	var nb NewsBrief
	var raw []byte
	var createdRaw sql.NullString
	if err := row.Scan(&nb.ID, &nb.ItemID, &nb.Sentiment, &nb.Brief, &raw, &createdRaw); err != nil {
		return NewsBrief{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		nb.CreatedAt = parsed
	}
	_ = json.Unmarshal(raw, &nb.Keywords)
	return nb, nil
}

func (r *Repository) HasBrief(ctx context.Context, itemID int64) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("repository_not_initialized")
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, `select exists(select 1 from news_briefs where item_id = ?)`, itemID).Scan(&exists)
	return exists, err
}

func (r *Repository) ListBriefsByKeywords(ctx context.Context, keywords []string, limit int) ([]BriefView, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository_not_initialized")
	}
	if len(keywords) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	clauses := []string{}
	args := []interface{}{}
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		args = append(args, "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
		clauses = append(clauses, "(lower(b.brief) like lower(?) or lower(i.title) like lower(?) or lower(i.content) like lower(?))")
	}
	if len(clauses) == 0 {
		return nil, nil
	}
	args = append(args, limit)
	query := `
		select b.item_id, i.title, i.link, i.published_at, b.sentiment, b.brief, b.keywords, b.created_at
		from news_briefs b
		join news_items i on i.id = b.item_id
		where ` + strings.Join(clauses, " or ") + `
		order by b.created_at desc
		limit ?
	`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []BriefView{}
	for rows.Next() {
		var view BriefView
		var raw []byte
		var publishedRaw sql.NullString
		var createdRaw sql.NullString
		if err := rows.Scan(&view.ItemID, &view.Title, &view.Link, &publishedRaw, &view.Sentiment, &view.Brief, &raw, &createdRaw); err != nil {
			return nil, err
		}
		if parsed, ok := db.ParseTime(publishedRaw); ok {
			view.PublishedAt = &parsed
		}
		if parsed, ok := db.ParseTime(createdRaw); ok {
			view.CreatedAt = parsed
		}
		_ = json.Unmarshal(raw, &view.Keywords)
		results = append(results, view)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// ──────────────────────────────────────────────
// News Events CRUD
// ──────────────────────────────────────────────

// EventQuery represents news event query parameters
type EventQuery struct {
	Page         int
	PageSize     int
	Domains      []EventDomain
	EventTypes   []string
	Sentiment    string
	DateFrom     *time.Time
	DateTo       *time.Time
	Keywords     []string
	SourceType   SourceType
	MinPriority  int
	SortBy       string
}

// CreateEvent creates a news event
func (r *Repository) CreateEvent(ctx context.Context, event NewsEvent) (NewsEvent, error) {
	if r == nil || r.db == nil {
		return NewsEvent{}, errors.New("repository_not_initialized")
	}

	domainsJSON, _ := json.Marshal(event.Domains)
	typesJSON, _ := json.Marshal(event.EventTypes)
	labelsJSON, _ := json.Marshal(event.Labels)
	relatedJSON, _ := json.Marshal(event.RelatedSources)
	var funnelJSON []byte
	if event.FunnelResult != nil {
		funnelJSON, _ = json.Marshal(event.FunnelResult)
	}

	row := r.db.QueryRowContext(ctx, `
		insert into news_events(raw_item_id, title, summary, content, source, source_type, url, published_at, processed_at,
			domains, event_types, labels, classify_confidence, classify_method, dedup_cluster_id, related_sources, funnel_result)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		returning id, created_at
	`, event.DBID, event.Title, event.Summary, event.Content, event.Source, event.SourceType, event.URL,
		event.PublishedAt, event.ProcessedAt, string(domainsJSON), string(typesJSON), string(labelsJSON),
		event.ClassifyConfidence, event.ClassifyMethod, event.DedupClusterID, string(relatedJSON), string(funnelJSON))

	var id int64
	var createdAtStr sql.NullString
	if err := row.Scan(&id, &createdAtStr); err != nil {
		return NewsEvent{}, err
	}

	event.DBID = id
	return event, nil
}

// ListEvents lists news events with filtering and pagination
func (r *Repository) ListEvents(ctx context.Context, query EventQuery) ([]NewsEvent, int, error) {
	if r == nil || r.db == nil {
		return nil, 0, errors.New("repository_not_initialized")
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	// Build WHERE clauses
	whereClauses := []string{"1=1"}
	args := []interface{}{}

	// Domain filter
	if len(query.Domains) > 0 {
		for _, d := range query.Domains {
			args = append(args, "%\""+string(d)+"\"%")
		}
		domainClauses := make([]string, len(query.Domains))
		for i := range query.Domains {
			domainClauses[i] = "domains like ?"
		}
		whereClauses = append(whereClauses, "("+strings.Join(domainClauses, " or ")+")")
	}

	// Event type filter
	if len(query.EventTypes) > 0 {
		for _, t := range query.EventTypes {
			args = append(args, "%\""+t+"\"%")
		}
		typeClauses := make([]string, len(query.EventTypes))
		for i := range query.EventTypes {
			typeClauses[i] = "event_types like ?"
		}
		whereClauses = append(whereClauses, "("+strings.Join(typeClauses, " or ")+")")
	}

	// Sentiment filter
	if query.Sentiment != "" {
		whereClauses = append(whereClauses, "json_extract(labels, '$.sentiment') like ?")
		sentimentVal := query.Sentiment
		switch sentimentVal {
		case "positive":
			args = append(args, "0.3%")
		case "negative":
			args = append(args, "-0.3%")
		case "neutral":
			args = append(args, "0.0%")
		default:
			args = append(args, "%")
		}
	}

	// Date range filter
	if query.DateFrom != nil {
		whereClauses = append(whereClauses, "published_at >= ?")
		args = append(args, query.DateFrom.Format("2006-01-02 15:04:05"))
	}
	if query.DateTo != nil {
		whereClauses = append(whereClauses, "published_at <= ?")
		args = append(args, query.DateTo.Format("2006-01-02 15:04:05"))
	}

	// Keywords filter
	if len(query.Keywords) > 0 {
		for _, kw := range query.Keywords {
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}
			args = append(args, "%"+kw+"%")
			whereClauses = append(whereClauses, "(lower(title) like lower(?) or lower(summary) like lower(?))")
			args = append(args, "%"+kw+"%")
		}
	}

	// Source type filter
	if query.SourceType != "" {
		whereClauses = append(whereClauses, "source_type = ?")
		args = append(args, query.SourceType)
	}

	// Min priority filter - handle NULL or invalid JSON gracefully
	if query.MinPriority > 0 {
		whereClauses = append(whereClauses, "(funnel_result IS NOT NULL AND json_valid(funnel_result) AND json_extract(funnel_result, '$.l2_priority') >= ?)")
		args = append(args, query.MinPriority)
	}

	whereClause := strings.Join(whereClauses, " and ")

	// Count total
	countQuery := "select count(*) from news_events where " + whereClause
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Build ORDER BY
	sortBy := "published_at"
	if query.SortBy != "" {
		switch query.SortBy {
		case "processed_at", "classify_confidence", "id":
			sortBy = query.SortBy
		}
	}

	// Query with pagination
	offset := (query.Page - 1) * query.PageSize
	args = append(args, query.PageSize, offset) // 修复：先 limit 再 offset

	selectQuery := `
		select id, raw_item_id, title, summary, content, source, source_type, url, published_at, processed_at,
			domains, event_types, labels, classify_confidence, classify_method, dedup_cluster_id, related_sources, funnel_result
		from news_events
		where ` + whereClause + `
		order by ` + sortBy + ` desc
		limit ? offset ?
	`

	log.Printf("[Repository.ListEvents] 查询SQL: %s", selectQuery)
	log.Printf("[Repository.ListEvents] 参数: %v", args)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		log.Printf("[Repository.ListEvents] 查询失败: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	events := make([]NewsEvent, 0) // 初始化为空 slice，而不是 nil
	for rows.Next() {
		var event NewsEvent
		var rawItemID sql.NullInt64
		var domainsJSON, typesJSON, labelsJSON, relatedJSON, funnelJSON []byte
		var publishedRaw, processedRaw sql.NullString

		if err := rows.Scan(&event.DBID, &rawItemID, &event.Title, &event.Summary, &event.Content, &event.Source, &event.SourceType,
			&event.URL, &publishedRaw, &processedRaw, &domainsJSON, &typesJSON, &labelsJSON,
			&event.ClassifyConfidence, &event.ClassifyMethod, &event.DedupClusterID, &relatedJSON, &funnelJSON); err != nil {
			log.Printf("[Repository.ListEvents] 扫描行失败: %v", err)
			return nil, 0, err
		}

		if parsed, ok := db.ParseTime(publishedRaw); ok {
			event.PublishedAt = parsed
		}
		if parsed, ok := db.ParseTime(processedRaw); ok {
			event.ProcessedAt = parsed
		}

		_ = json.Unmarshal(domainsJSON, &event.Domains)
		_ = json.Unmarshal(typesJSON, &event.EventTypes)
		_ = json.Unmarshal(labelsJSON, &event.Labels)
		_ = json.Unmarshal(relatedJSON, &event.RelatedSources)
		if len(funnelJSON) > 0 {
			_ = json.Unmarshal(funnelJSON, &event.FunnelResult)
		}

		// 将数据库 ID 转换为字符串赋值给 ID 字段，前端需要
		event.ID = fmt.Sprintf("%d", event.DBID)

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

// GetEventByID gets a news event by ID
func (r *Repository) GetEventByID(ctx context.Context, id int64) (NewsEvent, error) {
	if r == nil || r.db == nil {
		return NewsEvent{}, errors.New("repository_not_initialized")
	}

	row := r.db.QueryRowContext(ctx, `
		select id, raw_item_id, title, summary, content, source, source_type, url, published_at, processed_at,
			domains, event_types, labels, classify_confidence, classify_method, dedup_cluster_id, related_sources, funnel_result
		from news_events
		where id = ?
	`, id)

	var event NewsEvent
	var rawItemID sql.NullInt64
	var domainsJSON, typesJSON, labelsJSON, relatedJSON, funnelJSON []byte
	var publishedRaw, processedRaw sql.NullString

	if err := row.Scan(&event.DBID, &rawItemID, &event.Title, &event.Summary, &event.Content, &event.Source, &event.SourceType,
		&event.URL, &publishedRaw, &processedRaw, &domainsJSON, &typesJSON, &labelsJSON,
		&event.ClassifyConfidence, &event.ClassifyMethod, &event.DedupClusterID, &relatedJSON, &funnelJSON); err != nil {
		return NewsEvent{}, err
	}

	if parsed, ok := db.ParseTime(publishedRaw); ok {
		event.PublishedAt = parsed
	}
	if parsed, ok := db.ParseTime(processedRaw); ok {
		event.ProcessedAt = parsed
	}

	_ = json.Unmarshal(domainsJSON, &event.Domains)
	_ = json.Unmarshal(typesJSON, &event.EventTypes)
	_ = json.Unmarshal(labelsJSON, &event.Labels)
	_ = json.Unmarshal(relatedJSON, &event.RelatedSources)
	if len(funnelJSON) > 0 {
		_ = json.Unmarshal(funnelJSON, &event.FunnelResult)
	}

	// 将数据库 ID 转换为字符串赋值给 ID 字段，前端需要
	event.ID = fmt.Sprintf("%d", event.DBID)

	return event, nil
}

// ──────────────────────────────────────────────
// Anchors Management
// ──────────────────────────────────────────────

// SaveAnchors saves anchors
func (r *Repository) SaveAnchors(ctx context.Context, anchors []Anchor) error {
	if r == nil || r.db == nil {
		return errors.New("repository_not_initialized")
	}

	// Clear existing anchors
	_, _ = r.db.ExecContext(ctx, "delete from news_anchors")

	for _, a := range anchors {
		embeddingJSON, _ := json.Marshal(a.Embedding)
		_, err := r.db.ExecContext(ctx, `
			insert into news_anchors(type, text, embedding, weight, related_asset, updated_at)
			values (?, ?, ?, ?, ?, ?)
		`, string(a.Type), a.Text, string(embeddingJSON), a.Weight, a.RelatedAsset, a.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListAnchors lists all anchors
func (r *Repository) ListAnchors(ctx context.Context) ([]Anchor, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("repository_not_initialized")
	}

	rows, err := r.db.QueryContext(ctx, `
		select id, type, text, embedding, weight, related_asset, updated_at
		from news_anchors
		order by weight desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anchors []Anchor
	for rows.Next() {
		var a Anchor
		var embeddingJSON []byte
		var updatedAtRaw sql.NullString

		if err := rows.Scan(&a.ID, &a.Type, &a.Text, &embeddingJSON, &a.Weight, &a.RelatedAsset, &updatedAtRaw); err != nil {
			return nil, err
		}

		_ = json.Unmarshal(embeddingJSON, &a.Embedding)
		if parsed, ok := db.ParseTime(updatedAtRaw); ok {
			a.UpdatedAt = parsed
		}

		anchors = append(anchors, a)
	}

	return anchors, nil
}

// ──────────────────────────────────────────────
// Briefings Storage
// ──────────────────────────────────────────────

// SaveBriefing saves a briefing
func (r *Repository) SaveBriefing(ctx context.Context, brief Briefing) error {
	if r == nil || r.db == nil {
		return errors.New("repository_not_initialized")
	}

	macroJSON, _ := json.Marshal(brief.MacroOverview)
	impactJSON, _ := json.Marshal(brief.PortfolioImpact)
	oppJSON, _ := json.Marshal(brief.Opportunities)
	riskJSON, _ := json.Marshal(brief.RiskAlerts)
	conflictJSON, _ := json.Marshal(brief.ConflictSignals)
	monitorJSON, _ := json.Marshal(brief.MonitoringItems)
	statsJSON, _ := json.Marshal(map[string]int{
		"total_news_processed": brief.TotalNewsProcessed,
		"l1_passed":            brief.L1Passed,
		"l2_passed":            brief.L2Passed,
		"l3_analyzed":          brief.L3Analyzed,
	})

	_, err := r.db.ExecContext(ctx, `
		insert into news_analysis_briefings(trigger_type, period, generated_at,
			macro_overview, portfolio_impact, opportunities, risk_alerts,
			conflict_signals, monitoring_items, stats)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, string(brief.TriggerType), brief.Period, brief.GeneratedAt,
		string(macroJSON), string(impactJSON), string(oppJSON), string(riskJSON),
		string(conflictJSON), string(monitorJSON), string(statsJSON))

	return err
}

// GetLatestBriefing gets the latest briefing
func (r *Repository) GetLatestBriefing(ctx context.Context) (Briefing, error) {
	if r == nil || r.db == nil {
		return Briefing{}, errors.New("repository_not_initialized")
	}

	row := r.db.QueryRowContext(ctx, `
		select id, trigger_type, period, generated_at,
			macro_overview, portfolio_impact, opportunities, risk_alerts,
			conflict_signals, monitoring_items, stats
		from news_analysis_briefings
		order by generated_at desc
		limit 1
	`)

	var brief Briefing
	var macroJSON, impactJSON, oppJSON, riskJSON, conflictJSON, monitorJSON, statsJSON []byte
	var generatedAtRaw sql.NullString

	if err := row.Scan(&brief.ID, &brief.TriggerType, &brief.Period, &generatedAtRaw,
		&macroJSON, &impactJSON, &oppJSON, &riskJSON, &conflictJSON, &monitorJSON, &statsJSON); err != nil {
		return Briefing{}, err
	}

	if parsed, ok := db.ParseTime(generatedAtRaw); ok {
		brief.GeneratedAt = parsed
	}

	_ = json.Unmarshal(macroJSON, &brief.MacroOverview)
	_ = json.Unmarshal(impactJSON, &brief.PortfolioImpact)
	_ = json.Unmarshal(oppJSON, &brief.Opportunities)
	_ = json.Unmarshal(riskJSON, &brief.RiskAlerts)
	_ = json.Unmarshal(conflictJSON, &brief.ConflictSignals)
	_ = json.Unmarshal(monitorJSON, &brief.MonitoringItems)

	var stats map[string]int
	_ = json.Unmarshal(statsJSON, &stats)
	if stats != nil {
		brief.TotalNewsProcessed = stats["total_news_processed"]
		brief.L1Passed = stats["l1_passed"]
		brief.L2Passed = stats["l2_passed"]
		brief.L3Analyzed = stats["l3_analyzed"]
	}

	return brief, nil
}
