package api

import (
	"encoding/json"
	"net/http"

	"genFu/internal/tool"
)

type InvestmentHandler struct {
	registry *tool.Registry
}

func NewInvestmentHandler(registry *tool.Registry) *InvestmentHandler {
	return &InvestmentHandler{registry: registry}
}

func (h *InvestmentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.registry == nil {
		http.Error(w, "registry_not_initialized", http.StatusInternalServerError)
		return
	}
	var args map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	result, err := h.registry.Execute(r.Context(), tool.ToolCall{
		Name:      "investment",
		Arguments: args,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
