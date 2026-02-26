package analyze

import (
	"encoding/json"
	"log"
	"net/http"

	"genFu/internal/conversationlog"
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
	logRepo  *conversationlog.Repository
}

func NewSSEHandler(analyzer *Analyzer) *SSEHandler {
	return &SSEHandler{analyzer: analyzer}
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
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	sessionID := ""
	if h.logRepo != nil {
		title := conversationlog.BuildSessionTitle(req.SessionTitle, req.Prompt, "分析会话")
		session, err := h.logRepo.EnsureSession(r.Context(), req.SessionID, conversationlog.SceneAnalyze, title, "default")
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

	steps := []AnalyzeStep{}
	_, summary, err := h.analyzer.AnalyzeSteps(r.Context(), req, func(step AnalyzeStep) error {
		steps = append(steps, step)
		return writeEvent("step", step)
	})
	if err != nil {
		if h.logRepo != nil && sessionID != "" {
			reqRaw, _ := json.Marshal(req)
			_ = h.logRepo.AppendRun(r.Context(), sessionID, req.Prompt, reqRaw, nil, err.Error())
		}
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
	if h.logRepo != nil && sessionID != "" {
		reqRaw, _ := json.Marshal(req)
		respRaw, _ := json.Marshal(response)
		if err := h.logRepo.AppendRun(r.Context(), sessionID, req.Prompt, reqRaw, respRaw, ""); err != nil {
			log.Printf("conversation append failed: %v", err)
		}
	}

	_ = writeEvent("summary", response)
	_ = writeEvent("complete", map[string]bool{"done": true})
}
