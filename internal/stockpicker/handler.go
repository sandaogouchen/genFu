package stockpicker

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"genFu/internal/conversationlog"
)

// Handler HTTP处理器
type Handler struct {
	service          *Service
	guideRepo        *GuideRepository
	runRepo          *RunRepository
	conversationRepo *conversationlog.Repository
}

// NewHandler 创建处理器
func NewHandler(service *Service, guideRepo *GuideRepository) *Handler {
	h := &Handler{
		service:   service,
		guideRepo: guideRepo,
	}
	if service != nil {
		h.runRepo = service.runRepo
	}
	return h
}

func (h *Handler) SetConversationRepo(repo *conversationlog.Repository) {
	h.conversationRepo = repo
}

// ServeHTTP 处理HTTP请求
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StockPickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sessionID := ""
	if h.conversationRepo != nil {
		title := conversationlog.BuildSessionTitle(req.SessionTitle, req.Prompt, "智能选股会话")
		session, err := h.conversationRepo.EnsureSession(r.Context(), req.SessionID, conversationlog.SceneStockPicker, title, "default")
		if err != nil {
			log.Printf("conversation ensure failed: %v", err)
		} else {
			sessionID = session.ID
		}
	}

	resp, err := h.service.PickStocks(r.Context(), req)
	if err != nil {
		if h.conversationRepo != nil && sessionID != "" {
			reqRaw, _ := json.Marshal(req)
			_ = h.conversationRepo.AppendRun(r.Context(), sessionID, req.Prompt, reqRaw, nil, err.Error())
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if h.conversationRepo != nil && sessionID != "" {
		reqRaw, _ := json.Marshal(req)
		respRaw, _ := json.Marshal(resp)
		if err := h.conversationRepo.AppendRun(r.Context(), sessionID, req.Prompt, reqRaw, respRaw, ""); err != nil {
			log.Printf("conversation append failed: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetOperationGuide 获取股票操作指南
// GET /api/operation-guide?symbol=600519
func (h *Handler) GetOperationGuide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "missing symbol", http.StatusBadRequest)
		return
	}

	guide, err := h.guideRepo.GetLatestGuide(r.Context(), symbol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if guide == nil {
		http.Error(w, "guide not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(guide)
}

// GetOperationGuideByID 通过ID获取操作指南
// GET /api/operation-guide/{id}
func (h *Handler) GetOperationGuideByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL path
	// Assuming the path is /api/operation-guide/{id}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	idStr := parts[3]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	guide, err := h.guideRepo.GetGuideByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if guide == nil {
		http.Error(w, "guide not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(guide)
}

// ListOperationGuidesBySymbol 按标的代码返回历史指南
// GET /api/operation-guides?symbol=600519
func (h *Handler) ListOperationGuidesBySymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pickID := strings.TrimSpace(r.URL.Query().Get("pick_id"))
	if pickID != "" {
		guides, err := h.guideRepo.ListGuidesByPickID(r.Context(), pickID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := map[string]interface{}{
			"pick_id": pickID,
			"guides":  guides,
		}
		if h.runRepo != nil {
			record, err := h.runRepo.GetByPickID(r.Context(), pickID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if record != nil {
				resp["snapshot_summary"] = h.runRepo.BuildSummary(record)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}
	symbol := strings.TrimSpace(r.URL.Query().Get("symbol"))
	if symbol == "" {
		http.Error(w, "missing symbol or pick_id", http.StatusBadRequest)
		return
	}
	guides, err := h.guideRepo.ListGuidesBySymbol(r.Context(), symbol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(guides)
}
