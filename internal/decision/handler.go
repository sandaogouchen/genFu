package decision

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"genFu/internal/analyze"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	var req DecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	resp, err := h.service.Decide(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type SSEHandler struct {
	service *Service
}

func NewSSEHandler(service *Service) *SSEHandler {
	return &SSEHandler{service: service}
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
	var req DecisionRequest
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

	resp, err := h.service.Decide(r.Context(), req)
	if err != nil {
		_ = writeEvent("error", map[string]string{"error": err.Error()})
		return
	}

	// Save decision report to database
	if h.service.reports != nil {
		// Determine symbol and name from signals or use default
		symbol := "portfolio"
		name := "投资决策报告"
		if len(resp.Signals) > 0 && resp.Signals[0].Symbol != "" {
			symbol = resp.Signals[0].Symbol
			if resp.Signals[0].Name != "" {
				name = resp.Signals[0].Name
			}
		}

		// Build request with decision context
		reqPayload, _ := json.Marshal(map[string]interface{}{
			"account_id": req.AccountID,
			"report_ids": req.ReportIDs,
		})

		// Build steps from decision process
		steps := []analyze.AnalyzeStep{
			{
				Name:   "decision",
				Input:  string(reqPayload),
				Output: resp.Raw,
			},
		}

		// Add signals step
		if len(resp.Signals) > 0 {
			signalsJSON, _ := json.Marshal(resp.Signals)
			steps = append(steps, analyze.AnalyzeStep{
				Name:   "signals",
				Input:  "生成的交易信号",
				Output: string(signalsJSON),
			})
		}

		// Add executions step
		if len(resp.Executions) > 0 {
			execJSON, _ := json.Marshal(resp.Executions)
			steps = append(steps, analyze.AnalyzeStep{
				Name:   "executions",
				Input:  "执行结果",
				Output: string(execJSON),
			})
		}

		// Build summary from decision content
		summaryBytes, _ := json.Marshal(map[string]interface{}{
			"decision":   resp.Decision,
			"signals":    resp.Signals,
			"executions": resp.Executions,
		})
		summary := string(summaryBytes)

		// Create report request
		reportReq := analyze.AnalyzeRequest{
			Type:   "decision",
			Symbol: symbol,
			Name:   name,
			Meta:   req.Meta,
		}

		// Create report response
		reportResp := analyze.AnalyzeResponse{
			Type:    "decision",
			Symbol:  symbol,
			Name:    name,
			Steps:   steps,
			Summary: summary,
		}

		report, repoErr := h.service.reports.CreateReport(r.Context(), reportReq, reportResp)
		if repoErr != nil {
			log.Printf("failed to save decision report: %v", repoErr)
		} else {
			resp.ReportID = report.ID
			log.Printf("decision report saved with ID: %d", report.ID)

			// Generate title asynchronously
			if h.service.agent != nil {
				go func(reportID int64, summaryText string) {
					bgCtx := context.Background()
					tg := analyze.NewTitleGenerator(h.service.agent, h.service.reports)
					if err := tg.GenerateAndSave(bgCtx, reportID, summaryText); err != nil {
						log.Printf("title generation failed for decision report %d: %v", reportID, err)
					} else {
						log.Printf("title generated for decision report %d", reportID)
					}
				}(report.ID, resp.Raw)
			}
		}
	}

	_ = writeEvent("decision", resp.Decision)
	_ = writeEvent("signals", resp.Signals)
	_ = writeEvent("executions", resp.Executions)
	_ = writeEvent("complete", map[string]bool{"done": true})
}
