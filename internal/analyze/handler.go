package analyze

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	analyzer *Analyzer
}

func NewHandler(analyzer *Analyzer) *Handler {
	return &Handler{analyzer: analyzer}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	resp, err := h.analyzer.Analyze(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type SSEHandler struct {
	analyzer *Analyzer
}

func NewSSEHandler(analyzer *Analyzer) *SSEHandler {
	return &SSEHandler{analyzer: analyzer}
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream_not_supported", http.StatusInternalServerError)
		return
	}
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writeEvent := func(event string, data interface{}) error {
		payload, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
			return err
		}
		if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	steps := []AnalyzeStep{}
	_, summary, err := h.analyzer.AnalyzeSteps(r.Context(), req, func(step AnalyzeStep) error {
		steps = append(steps, step)
		return writeEvent("step", step)
	})
	if err != nil {
		payload := map[string]string{"error": err.Error()}
		if stepErr, ok := err.(StepError); ok {
			payload["step"] = stepErr.Step
		}
		log.Printf("sse.analyze error=%s step=%s", payload["error"], payload["step"])
		_ = writeEvent("error", payload)
		return
	}
	response := AnalyzeResponse{
		Type:    req.Type,
		Symbol:  req.Symbol,
		Name:    req.Name,
		Steps:   steps,
		Summary: summary,
	}

	// Save report to database
	if h.analyzer != nil && h.analyzer.repo != nil {
		report, repoErr := h.analyzer.repo.CreateReport(r.Context(), req, response)
		if repoErr != nil {
			log.Printf("failed to save report: %v", repoErr)
		} else {
			response.ReportID = report.ID
			log.Printf("report saved with ID: %d", report.ID)

			// Generate title asynchronously
			if h.analyzer.titleGenerator != nil && strings.TrimSpace(summary) != "" {
				go func(reportID int64, summaryText string) {
					// Use context.Background() for async operation to avoid cancellation
					bgCtx := context.Background()
					if err := h.analyzer.titleGenerator.GenerateAndSave(bgCtx, reportID, summaryText); err != nil {
						log.Printf("title generation failed for report %d: %v", reportID, err)
					} else {
						log.Printf("title generated for report %d", reportID)
					}
				}(report.ID, summary)
			}
		}
	}

	_ = writeEvent("summary", response)
	_ = writeEvent("complete", map[string]bool{"done": true})
}

// ListHandler handles report list queries
type ListHandler struct {
	repo *Repository
}

// NewListHandler creates a new list handler
func NewListHandler(repo *Repository) *ListHandler {
	return &ListHandler{repo: repo}
}

// ListReportsResponse is the response structure for list reports API
type ListReportsResponse struct {
	Items    []ReportListItem `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

func (h *ListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	reportType := r.URL.Query().Get("type")
	search := r.URL.Query().Get("search")

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Get reports
	items, err := h.repo.ListReports(r.Context(), reportType, search, page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get total count
	total, err := h.repo.CountReports(r.Context(), reportType, search)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := ListReportsResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// DetailHandler handles report detail queries
type DetailHandler struct {
	repo *Repository
}

// NewDetailHandler creates a new detail handler
func NewDetailHandler(repo *Repository) *DetailHandler {
	return &DetailHandler{repo: repo}
}

func (h *DetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/reports/")
	if path == "" {
		http.Error(w, "missing_report_id", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "invalid_report_id", http.StatusBadRequest)
		return
	}

	report, err := h.repo.GetReport(r.Context(), id)
	if err != nil {
		http.Error(w, "report_not_found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
