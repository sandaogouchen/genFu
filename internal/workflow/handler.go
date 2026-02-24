package workflow

import (
	"encoding/json"
	"net/http"
)

type StockHandler struct {
	service *StockWorkflow
}

func NewStockHandler(service *StockWorkflow) *StockHandler {
	return &StockHandler{service: service}
}

func (h *StockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	var req StockWorkflowInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}
	resp, err := h.service.Run(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
