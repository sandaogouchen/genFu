package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	decisionpkg "genFu/internal/decision"
	"genFu/internal/generate"
	"genFu/internal/message"
	stockpickerpkg "genFu/internal/stockpicker"
)

type decisionWorkflow interface {
	Decide(ctx context.Context, req decisionpkg.DecisionRequest) (decisionpkg.DecisionResponse, error)
}

type stockpickerWorkflow interface {
	PickStocks(ctx context.Context, req stockpickerpkg.StockPickRequest) (stockpickerpkg.StockPickResponse, error)
}

func (s *Service) runDecisionWorkflow(ctx context.Context, req generate.GenerateRequest, sessionID string, route RouteDecision) (message.Message, error) {
	if s == nil || s.decisionSvc == nil {
		return message.Message{}, errors.New("decision_service_not_ready")
	}
	lastUser := strings.TrimSpace(lastUserMessage(req.Messages))
	decisionReq := decisionpkg.DecisionRequest{
		Meta:      req.Meta,
		SessionID: sessionID,
		Prompt:    lastUser,
	}
	if title := strings.TrimSpace(req.Meta["session_title"]); title != "" {
		decisionReq.SessionTitle = title
	}
	if accountID, ok := resolveAccountID(req.Meta, route.Slots); ok {
		decisionReq.AccountID = accountID
	}
	if ids, ok := resolveReportIDs(req.Meta, route.Slots); ok {
		decisionReq.ReportIDs = ids
	}
	resp, err := s.decisionSvc.Decide(ctx, decisionReq)
	if err != nil {
		return message.Message{}, err
	}
	content := marshalPretty(map[string]interface{}{
		"workflow":     "decision",
		"decision":     resp.Decision,
		"signals":      resp.Signals,
		"executions":   resp.Executions,
		"tool_results": resp.ToolResults,
	})
	return message.Message{Role: message.RoleAssistant, Content: content}, nil
}

func (s *Service) runStockpickerWorkflow(ctx context.Context, req generate.GenerateRequest, sessionID string, route RouteDecision) (message.Message, error) {
	if s == nil || s.stockpickerSvc == nil {
		return message.Message{}, errors.New("stockpicker_service_not_ready")
	}
	lastUser := strings.TrimSpace(lastUserMessage(req.Messages))
	stockReq, err := buildStockpickerRequest(req.Meta, route.Slots)
	if err != nil {
		return message.Message{}, err
	}
	stockReq.SessionID = sessionID
	stockReq.Prompt = lastUser
	if title := strings.TrimSpace(req.Meta["session_title"]); title != "" {
		stockReq.SessionTitle = title
	}
	resp, err := s.stockpickerSvc.PickStocks(ctx, stockReq)
	if err != nil {
		return message.Message{}, err
	}
	content := marshalPretty(map[string]interface{}{
		"workflow": "stockpicker",
		"result":   resp,
	})
	return message.Message{Role: message.RoleAssistant, Content: content}, nil
}

func buildStockpickerRequest(meta map[string]string, slots RouteSlots) (stockpickerpkg.StockPickRequest, error) {
	var req stockpickerpkg.StockPickRequest
	if accountID, ok := resolveAccountID(meta, slots); ok {
		req.AccountID = accountID
	}
	req.TopN = 5
	if topN, ok := resolveTopN(meta, slots); ok {
		req.TopN = clampTopN(topN)
	}

	dateFrom, hasFrom, err := resolveDate(meta, slots.DateFrom, "date_from")
	if err != nil {
		return stockpickerpkg.StockPickRequest{}, err
	}
	dateTo, hasTo, err := resolveDate(meta, slots.DateTo, "date_to")
	if err != nil {
		return stockpickerpkg.StockPickRequest{}, err
	}
	if hasFrom {
		req.DateFrom = dateFrom
	}
	if hasTo {
		req.DateTo = dateTo
	}
	return req, nil
}

func resolveAccountID(meta map[string]string, slots RouteSlots) (int64, bool) {
	if n, ok := parseInt64String(meta["account_id"]); ok && n > 0 {
		return n, true
	}
	if slots.AccountID > 0 {
		return slots.AccountID, true
	}
	return 0, false
}

func resolveTopN(meta map[string]string, slots RouteSlots) (int, bool) {
	if n, ok := parseIntString(meta["top_n"]); ok {
		return n, true
	}
	if slots.TopN > 0 {
		return slots.TopN, true
	}
	return 0, false
}

func resolveReportIDs(meta map[string]string, slots RouteSlots) ([]int64, bool) {
	if ids := parseReportIDsString(meta["report_ids"]); len(ids) > 0 {
		return ids, true
	}
	if len(slots.ReportIDs) > 0 {
		return clampReportIDs(slots.ReportIDs), true
	}
	return nil, false
}

func resolveDate(meta map[string]string, slot string, key string) (time.Time, bool, error) {
	if value := strings.TrimSpace(meta[key]); value != "" {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid_%s", key)
		}
		return parsed, true, nil
	}
	if strings.TrimSpace(slot) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(slot))
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid_%s", key)
		}
		return parsed, true, nil
	}
	return time.Time{}, false, nil
}

func parseIntString(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseInt64String(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseReportIDsString(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var arr []int64
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			return clampReportIDs(arr)
		}
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		out = append(out, n)
	}
	return clampReportIDs(out)
}

func clampTopN(raw int) int {
	if raw < 1 {
		return 1
	}
	if raw > 20 {
		return 20
	}
	return raw
}

func marshalPretty(payload interface{}) string {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", payload)
	}
	return string(raw)
}
