package conversationlog

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type SessionsHandler struct {
	repo *Repository
}

type SessionItemHandler struct {
	repo *Repository
}

type RunsHandler struct {
	repo *Repository
}

type CreateSessionRequest struct {
	Scene string `json:"scene"`
	Title string `json:"title,omitempty"`
}

type RenameSessionRequest struct {
	Title string `json:"title"`
}

func NewSessionsHandler(repo *Repository) *SessionsHandler {
	return &SessionsHandler{repo: repo}
}

func NewSessionItemHandler(repo *Repository) *SessionItemHandler {
	return &SessionItemHandler{repo: repo}
}

func NewRunsHandler(repo *Repository) *RunsHandler {
	return &RunsHandler{repo: repo}
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *SessionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		http.Error(w, "conversation_repo_not_ready", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid_request", http.StatusBadRequest)
			return
		}
		session, err := h.repo.CreateSession(r.Context(), req.Scene, req.Title, defaultUserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "invalid_scene") {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, session)
	case http.MethodGet:
		scene := r.URL.Query().Get("scene")
		limit := 50
		offset := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}
		if raw := r.URL.Query().Get("offset"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
				offset = n
			}
		}
		sessions, err := h.repo.ListSessions(r.Context(), scene, limit, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"items":  sessions,
			"limit":  limit,
			"offset": offset,
		})
	default:
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SessionItemHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		http.Error(w, "conversation_repo_not_ready", http.StatusServiceUnavailable)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/conversations/sessions/")
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		http.Error(w, "missing_session_id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var req RenameSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid_request", http.StatusBadRequest)
			return
		}
		if err := h.repo.RenameSession(r.Context(), sessionID, req.Title); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "session_not_found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		session, err := h.repo.GetSession(r.Context(), sessionID)
		if err != nil {
			http.Error(w, "session_not_found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, session)
	case http.MethodDelete:
		if err := h.repo.SoftDeleteSession(r.Context(), sessionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "session_not_found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
	}
}

func (h *RunsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		http.Error(w, "conversation_repo_not_ready", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		http.Error(w, "missing_session_id", http.StatusBadRequest)
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	runs, err := h.repo.ListRuns(r.Context(), sessionID, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "session_not_found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": runs,
		"limit": limit,
	})
}
