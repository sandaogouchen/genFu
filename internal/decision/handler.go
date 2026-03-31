package decision

import (
	"encoding/json"
	"log"
	"net/http"

	"genFu/internal/conversationlog"
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
	logRepo *conversationlog.Repository
}

func NewSSEHandler(service *Service) *SSEHandler {
	return &SSEHandler{service: service}
}

func (h *SSEHandler) SetConversationRepo(repo *conversationlog.Repository) {
	h.logRepo = repo
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
	sessionID := ""
	if h.logRepo != nil {
		title := conversationlog.BuildSessionTitle(req.SessionTitle, req.Prompt, "决策会话")
		session, err := h.logRepo.EnsureSession(r.Context(), req.SessionID, conversationlog.SceneDecision, title, "default")
		if err != nil {
			log.Printf("conversation ensure failed: %v", err)
		} else {
			sessionID = session.ID
		}
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
		if h.logRepo != nil && sessionID != "" {
			reqRaw, _ := json.Marshal(req)
			_ = h.logRepo.AppendRun(r.Context(), sessionID, req.Prompt, reqRaw, nil, err.Error())
		}
		_ = writeEvent("error", map[string]string{"error": err.Error()})
		return
	}
	if h.logRepo != nil && sessionID != "" {
		reqRaw, _ := json.Marshal(req)
		respRaw, _ := json.Marshal(resp)
		if err := h.logRepo.AppendRun(r.Context(), sessionID, req.Prompt, reqRaw, respRaw, ""); err != nil {
			log.Printf("conversation append failed: %v", err)
		}
	}

	_ = writeEvent("decision", resp.Decision)
	_ = writeEvent("risk_budget", resp.RiskBudget)
	_ = writeEvent("planned_orders", resp.PlannedOrders)
	_ = writeEvent("guarded_orders", resp.GuardedOrders)
	_ = writeEvent("signals", resp.Signals)
	_ = writeEvent("executions", resp.Executions)
	if resp.Review != nil {
		_ = writeEvent("review", resp.Review)
	}
	if len(resp.Warnings) > 0 {
		_ = writeEvent("warnings", resp.Warnings)
	}
	_ = writeEvent("complete", map[string]bool{"done": true})
}
