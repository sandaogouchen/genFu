package news

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Handler represents news HTTP handler
type Handler struct {
	repo     *Repository
	pipeline *Pipeline
}

// NewHandler creates a new news handler
func NewHandler(repo *Repository, pipeline *Pipeline) *Handler {
	return &Handler{
		repo:     repo,
		pipeline: pipeline,
	}
}

// ──────────────────────────────────────────────
// GET /api/news/events - List news events with pagination
// ──────────────────────────────────────────────

// ListEventsHandler handles GET /api/news/events
func (h *Handler) ListEventsHandler(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		http.Error(w, "repository not initialized", http.StatusInternalServerError)
		return
	}

	// Parse query parameters
	query := EventQuery{
		Page:    1,
		PageSize: 20,
	}

	if page := r.URL.Query().Get("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			query.Page = p
		}
	}

	if pageSize := r.URL.Query().Get("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil && ps > 0 && ps <= 100 {
			query.PageSize = ps
		}
	}

	if domain := r.URL.Query().Get("domain"); domain != "" {
		domains := strings.Split(domain, ",")
		for _, d := range domains {
			d = strings.TrimSpace(d)
			if d != "" {
				query.Domains = append(query.Domains, EventDomain(d))
			}
		}
	}

	if eventType := r.URL.Query().Get("event_type"); eventType != "" {
		types := strings.Split(eventType, ",")
		for _, t := range types {
			t = strings.TrimSpace(t)
			if t != "" {
				query.EventTypes = append(query.EventTypes, t)
			}
		}
	}

	if sentiment := r.URL.Query().Get("sentiment"); sentiment != "" {
		query.Sentiment = sentiment
	}

	if dateFrom := r.URL.Query().Get("date_from"); dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			query.DateFrom = &t
		}
	}

	if dateTo := r.URL.Query().Get("date_to"); dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			query.DateTo = &t
		}
	}

	if keywords := r.URL.Query().Get("keywords"); keywords != "" {
		kws := strings.Split(keywords, ",")
		for _, kw := range kws {
			kw = strings.TrimSpace(kw)
			if kw != "" {
				query.Keywords = append(query.Keywords, kw)
			}
		}
	}

	if sourceType := r.URL.Query().Get("source_type"); sourceType != "" {
		query.SourceType = SourceType(sourceType)
	}

	if minPriority := r.URL.Query().Get("min_priority"); minPriority != "" {
		if p, err := strconv.Atoi(minPriority); err == nil && p >= 1 && p <= 5 {
			query.MinPriority = p
		}
	}

	if sortBy := r.URL.Query().Get("sort_by"); sortBy != "" {
		query.SortBy = sortBy
	}

	// Query events
	events, total, err := h.repo.ListEvents(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response
	response := map[string]interface{}{
		"items":     events,
		"total":     total,
		"page":      query.Page,
		"page_size": query.PageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ──────────────────────────────────────────────
// GET /api/news/events/{id} - Get single news event
// ──────────────────────────────────────────────

// GetEventHandler handles GET /api/news/events/{id}
func (h *Handler) GetEventHandler(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		http.Error(w, "repository not initialized", http.StatusInternalServerError)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/news/events/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	event, err := h.repo.GetEventByID(r.Context(), id)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

// ──────────────────────────────────────────────
// GET /api/news/briefing - Get latest briefing
// ──────────────────────────────────────────────

// GetBriefingHandler handles GET /api/news/briefing
func (h *Handler) GetBriefingHandler(w http.ResponseWriter, r *http.Request) {
	// Try to get from pipeline store first
	if h.pipeline != nil {
		store := h.pipeline.GetStore()
		if store != nil {
			brief := store.GetLatestBriefing()
			if brief != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(brief)
				return
			}
		}
	}

	// Fall back to repository
	if h.repo != nil {
		brief, err := h.repo.GetLatestBriefing(r.Context())
		if err == nil && brief.ID != "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(brief)
			return
		}
	}

	// Return empty briefing if none found
	http.Error(w, "no briefing available", http.StatusNotFound)
}

// ──────────────────────────────────────────────
// POST /api/news/analyze - Trigger manual analysis
// ──────────────────────────────────────────────

// TriggerAnalysisHandler handles POST /api/news/analyze
func (h *Handler) TriggerAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	if h.pipeline == nil {
		http.Error(w, "pipeline not initialized", http.StatusInternalServerError)
		return
	}

	// Run pipeline
	result, err := h.pipeline.Run(r.Context(), TriggerManual)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ──────────────────────────────────────────────
// Helper functions
// ──────────────────────────────────────────────

// NewListEventsHandler creates a handler for listing events
func NewListEventsHandler(repo *Repository, pipeline *Pipeline) http.HandlerFunc {
	h := NewHandler(repo, pipeline)
	return h.ListEventsHandler
}

// NewGetEventHandler creates a handler for getting a single event
func NewGetEventHandler(repo *Repository, pipeline *Pipeline) http.HandlerFunc {
	h := NewHandler(repo, pipeline)
	return h.GetEventHandler
}

// NewGetBriefingHandler creates a handler for getting the latest briefing
func NewGetBriefingHandler(repo *Repository, pipeline *Pipeline) http.HandlerFunc {
	h := NewHandler(repo, pipeline)
	return h.GetBriefingHandler
}

// NewTriggerAnalysisHandler creates a handler for triggering analysis
func NewTriggerAnalysisHandler(repo *Repository, pipeline *Pipeline) http.HandlerFunc {
	h := NewHandler(repo, pipeline)
	return h.TriggerAnalysisHandler
}
