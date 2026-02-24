package workflow

import (
	"encoding/json"
	"log"
	"net/http"

	"genFu/internal/analyze"
)

type StockSSEHandler struct {
	service *StockWorkflow
	repo    *analyze.Repository
}

func NewStockSSEHandler(service *StockWorkflow) *StockSSEHandler {
	return &StockSSEHandler{service: service}
}

// SetAnalyzeRepo sets the analyze repository for report storage
func (h *StockSSEHandler) SetAnalyzeRepo(repo *analyze.Repository) {
	h.repo = repo
}

func (h *StockSSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream_not_supported", http.StatusInternalServerError)
		return
	}
	var req StockWorkflowInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	resp, err := h.service.Run(r.Context(), req)
	if err != nil {
		writeSSE(w, flusher, "error", map[string]string{"error": err.Error()})
		return
	}
	writeSSE(w, flusher, "holdings", resp.Holdings)
	writeSSE(w, flusher, "holdings_market", resp.HoldingsMarket)
	writeSSE(w, flusher, "target_market", resp.TargetMarket)
	writeSSE(w, flusher, "news_summary", resp.News)
	writeSSE(w, flusher, "bull", map[string]string{"content": resp.BullAnalysis})
	writeSSE(w, flusher, "bear", map[string]string{"content": resp.BearAnalysis})
	writeSSE(w, flusher, "debate", map[string]string{"content": resp.DebateAnalysis})
	writeSSE(w, flusher, "summary", map[string]string{"content": resp.Summary})
	writeSSE(w, flusher, "complete", map[string]bool{"done": true})

	// Save workflow report to database
	if h.repo != nil && resp.Summary != "" {
		// Build summary from workflow outputs
		summaryBytes, _ := json.Marshal(map[string]interface{}{
			"holdings":        resp.Holdings,
			"holdings_market": resp.HoldingsMarket,
			"target_market":   resp.TargetMarket,
			"news":            resp.News,
			"bull_analysis":   resp.BullAnalysis,
			"bear_analysis":   resp.BearAnalysis,
			"debate_analysis": resp.DebateAnalysis,
			"summary":         resp.Summary,
		})
		summary := string(summaryBytes)

		// Determine symbol from request or target market
		symbol := req.Symbol
		name := req.Name
		if symbol == "" && resp.TargetMarket.Symbol != "" {
			symbol = resp.TargetMarket.Symbol
			if resp.TargetMarket.Name != "" {
				name = resp.TargetMarket.Name
			}
		}
		if symbol == "" {
			symbol = "portfolio"
			name = "股票工作流报告"
		}

		// Create report
		reportReq := analyze.AnalyzeRequest{
			Type:   "workflow",
			Symbol: symbol,
			Name:   name,
		}
		reportResp := analyze.AnalyzeResponse{
			Type:    "workflow",
			Symbol:  symbol,
			Name:    name,
			Summary: summary,
		}

		report, repoErr := h.repo.CreateReport(r.Context(), reportReq, reportResp)
		if repoErr != nil {
			log.Printf("failed to save workflow report: %v", repoErr)
		} else {
			log.Printf("workflow report saved with ID: %d", report.ID)

			// Generate title asynchronously
			go func(reportID int64, summaryText string) {
				// For workflow, we'll skip title generation for now
				// In a production system, you'd want to inject an agent here
				log.Printf("workflow report %d created (title generation skipped)", reportID)
			}(report.ID, resp.Summary)
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("event: " + event + "\n"))
	_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
	flusher.Flush()
}
