package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Handler HTTP handler for dashboard endpoints.
type Handler struct {
	dataSvc   *DataService
	generator *HTMLGenerator
}

// NewHandler creates a new dashboard HTTP handler.
func NewHandler(dataSvc *DataService, generator *HTMLGenerator) *Handler {
	return &Handler{
		dataSvc:   dataSvc,
		generator: generator,
	}
}

// RegisterRoutes registers all dashboard HTTP routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dashboard", h.handleDashboardHTML)
	mux.HandleFunc("GET /api/dashboard/data", h.handleDashboardData)
	mux.HandleFunc("GET /api/dashboard/summary", h.handleDashboardSummary)
}

// parseAccountID extracts account_id from query params, defaults to 0 (triggers default account).
func parseAccountID(r *http.Request) int64 {
	s := r.URL.Query().Get("account_id")
	if s == "" {
		return 0
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// parseDashboardOptions extracts dashboard options from query params.
func parseDashboardOptions(r *http.Request) DashboardOptions {
	opts := DashboardOptions{
		ValuationDays: 30,
		ColorScheme:   "cn",
		IncludeCash:   true,
		GroupBy:       "industry",
	}

	if days := r.URL.Query().Get("days"); days != "" {
		if d, err := strconv.Atoi(days); err == nil && d > 0 && d <= 365 {
			opts.ValuationDays = d
		}
	}
	if scheme := r.URL.Query().Get("color_scheme"); scheme == "us" || scheme == "cn" {
		opts.ColorScheme = scheme
	}
	if cash := r.URL.Query().Get("include_cash"); cash == "false" {
		opts.IncludeCash = false
	}
	if groupBy := r.URL.Query().Get("group_by"); groupBy == "asset_type" || groupBy == "industry" {
		opts.GroupBy = groupBy
	}
	return opts
}

// handleDashboardHTML serves the self-contained HTML dashboard page.
func (h *Handler) handleDashboardHTML(w http.ResponseWriter, r *http.Request) {
	accountID := parseAccountID(r)
	opts := parseDashboardOptions(r)

	data, err := h.dataSvc.BuildDashboardData(r.Context(), accountID, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if err := h.generator.GenerateHTML(w, data); err != nil {
		// Headers already sent; log and move on.
		fmt.Printf("[dashboard] template render error: %v\n", err)
	}
}

// handleDashboardData returns the full structured dashboard JSON data.
func (h *Handler) handleDashboardData(w http.ResponseWriter, r *http.Request) {
	accountID := parseAccountID(r)
	opts := parseDashboardOptions(r)

	data, err := h.dataSvc.BuildDashboardData(r.Context(), accountID, opts)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, data)
}

// handleDashboardSummary returns only the KPI summary.
func (h *Handler) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	accountID := parseAccountID(r)
	opts := parseDashboardOptions(r)

	data, err := h.dataSvc.BuildDashboardData(r.Context(), accountID, opts)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, data.Summary)
}

// writeJSON writes a JSON response with 200 status.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Printf("[dashboard] json encode error: %v\n", err)
	}
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
