package chat

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"genFu/internal/analyze"
	"genFu/internal/generate"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	NewSSEHandler(h.service).ServeHTTP(w, r)
}

type SSEHandler struct {
	service *Service
	repo   *analyze.Repository
}

func NewSSEHandler(service *Service) *SSEHandler {
	return &SSEHandler{service: service}
}

// SetAnalyzeRepo sets the analyze repository for report storage
func (h *SSEHandler) SetAnalyzeRepo(repo *analyze.Repository) {
	h.repo = repo
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
	var req generate.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	ch, _, err := h.service.ChatStream(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var lastMessage string
	var sessionID string

	for evt := range ch {
		payload, err := json.Marshal(evt)
		if err != nil {
			continue
		}
		if _, err := w.Write([]byte("event: " + evt.Type + "\n")); err != nil {
			return
		}
		if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
			return
		}
		flusher.Flush()

		// Capture session ID and last message for report storage
		if evt.Type == "session" {
			sessionID = evt.Delta
		}
		if evt.Type == "message" && evt.Message != nil {
			lastMessage = evt.Message.Content
		}
	}

	// Save chat report to database
	if h.repo != nil && sessionID != "" && lastMessage != "" {
		// Create a summary from the last assistant message
		summaryBytes, _ := json.Marshal(map[string]interface{}{
			"session_id": sessionID,
			"content":    lastMessage,
		})
		summary := string(summaryBytes)

		// Create report
		reportReq := analyze.AnalyzeRequest{
			Type:   "chat",
			Symbol: "chat",
			Name:   "对话记录",
		}
		reportResp := analyze.AnalyzeResponse{
			Type:    "chat",
			Symbol:  "chat",
			Name:    "对话记录",
			Summary: summary,
		}

		report, repoErr := h.repo.CreateReport(r.Context(), reportReq, reportResp)
		if repoErr != nil {
			log.Printf("failed to save chat report: %v", repoErr)
		} else {
			log.Printf("chat report saved with ID: %d", report.ID)

			// Generate title asynchronously using the last message
			if h.service.model != nil {
				go func(reportID int64, summaryText string) {
					// For chat, we'll create a simple title generator
					// Since we don't have direct access to agent, we'll skip title generation for now
					// In a production system, you'd want to inject an agent here
					log.Printf("chat report %d created (title generation skipped)", reportID)
				}(report.ID, lastMessage)
			}
		}
	}
}

type HistoryHandler struct {
	service *Service
}

func NewHistoryHandler(service *Service) *HistoryHandler {
	return &HistoryHandler{service: service}
}

func (h *HistoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	msgs, err := h.service.History(r.Context(), sessionID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}
