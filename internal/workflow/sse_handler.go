package workflow

import (
	"encoding/json"
	"log"
	"net/http"

	"genFu/internal/conversationlog"
)

type StockSSEHandler struct {
	service *StockWorkflow
	logRepo *conversationlog.Repository
}

func NewStockSSEHandler(service *StockWorkflow) *StockSSEHandler {
	return &StockSSEHandler{service: service}
}

func (h *StockSSEHandler) SetConversationRepo(repo *conversationlog.Repository) {
	h.logRepo = repo
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
	sessionID := ""
	if h.logRepo != nil {
		title := conversationlog.BuildSessionTitle(req.SessionTitle, req.Prompt, "工作流会话")
		session, err := h.logRepo.EnsureSession(r.Context(), req.SessionID, conversationlog.SceneWorkflow, title, "default")
		if err != nil {
			log.Printf("conversation ensure failed: %v", err)
		} else {
			sessionID = session.ID
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	resp, err := h.service.Run(r.Context(), req)
	if err != nil {
		if h.logRepo != nil && sessionID != "" {
			reqRaw, _ := json.Marshal(req)
			_ = h.logRepo.AppendRun(r.Context(), sessionID, req.Prompt, reqRaw, nil, err.Error())
		}
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
	if h.logRepo != nil && sessionID != "" {
		reqRaw, _ := json.Marshal(req)
		respRaw, _ := json.Marshal(resp)
		if err := h.logRepo.AppendRun(r.Context(), sessionID, req.Prompt, reqRaw, respRaw, ""); err != nil {
			log.Printf("conversation append failed: %v", err)
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
