package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type IntentType string

const (
	IntentGeneralChat IntentType = "general_chat"
	IntentPortfolio   IntentType = "portfolio_ops"
	IntentDecision    IntentType = "decision"
	IntentStockPicker IntentType = "stockpicker"
)

type WorkflowType string

const (
	WorkflowChatGeneral    WorkflowType = "workflow_chat_general"
	WorkflowChatPortfolio  WorkflowType = "workflow_chat_portfolio"
	WorkflowDomainDecision WorkflowType = "workflow_domain_decision"
	WorkflowDomainPicker   WorkflowType = "workflow_domain_stockpicker"
)

type RouteSlots struct {
	AccountID int64   `json:"account_id,omitempty"`
	TopN      int     `json:"top_n,omitempty"`
	DateFrom  string  `json:"date_from,omitempty"`
	DateTo    string  `json:"date_to,omitempty"`
	ReportIDs []int64 `json:"report_ids,omitempty"`
}

type RouteInput struct {
	LastUserMessage string
	Meta            map[string]string
	SessionSummary  string
}

type RouteDecision struct {
	Intent       IntentType   `json:"intent"`
	Workflow     WorkflowType `json:"workflow"`
	Confidence   float64      `json:"confidence"`
	Reason       string       `json:"reason,omitempty"`
	Slots        RouteSlots   `json:"slots,omitempty"`
	AllowedTools []string     `json:"allowed_tools,omitempty"`
	Fallback     bool         `json:"fallback,omitempty"`
}

type IntentRouter interface {
	Route(ctx context.Context, input RouteInput) (RouteDecision, error)
}

type IntentRouterAgent struct {
	model     model.ToolCallingChatModel
	threshold float64
}

func NewIntentRouterAgent(m model.ToolCallingChatModel) *IntentRouterAgent {
	return &IntentRouterAgent{
		model:     m,
		threshold: 0.60,
	}
}

func (a *IntentRouterAgent) Route(ctx context.Context, input RouteInput) (RouteDecision, error) {
	text := strings.TrimSpace(input.LastUserMessage)
	if strings.HasPrefix(strings.ToLower(text), "call:") || strings.HasPrefix(strings.ToLower(text), "tool:") {
		return buildRouteDecision(IntentPortfolio, 1.0, "explicit_tool_prefix", RouteSlots{}, false), nil
	}
	raw, err := a.classifyByLLM(ctx, input)
	if err != nil {
		return fallbackRoute("intent_classification_failed"), nil
	}
	intent := normalizeIntent(raw.Intent)
	if intent == "" {
		return fallbackRoute("invalid_intent"), nil
	}
	conf := raw.Confidence
	if conf < 0 {
		conf = 0
	}
	if conf > 1 {
		conf = 1
	}
	slots := sanitizeRouteSlots(raw.Slots)
	decision := buildRouteDecision(intent, conf, strings.TrimSpace(raw.Reason), slots, false)
	if conf < a.threshold {
		fb := fallbackRoute("low_confidence")
		fb.Confidence = conf
		return fb, nil
	}
	return decision, nil
}

type llmRouteDecision struct {
	Intent     string                 `json:"intent"`
	Confidence float64                `json:"confidence"`
	Reason     string                 `json:"reason"`
	Slots      map[string]interface{} `json:"slots"`
}

func (a *IntentRouterAgent) classifyByLLM(ctx context.Context, input RouteInput) (llmRouteDecision, error) {
	if a == nil || a.model == nil {
		return llmRouteDecision{}, errors.New("model_not_initialized")
	}
	metaJSON, _ := json.Marshal(input.Meta)
	userPrompt := fmt.Sprintf(
		"用户输入: %s\n会话摘要: %s\n请求Meta: %s\n请输出路由JSON。",
		strings.TrimSpace(input.LastUserMessage),
		strings.TrimSpace(input.SessionSummary),
		string(metaJSON),
	)
	systemPrompt := `你是意图路由器。仅输出JSON，不要Markdown。合法intent仅可为:
- general_chat
- portfolio_ops
- decision
- stockpicker

必须输出:
{
  "intent":"...",
  "confidence":0到1之间数字,
  "reason":"...",
  "slots":{
    "account_id":数字,
    "top_n":数字,
    "date_from":"YYYY-MM-DD",
    "date_to":"YYYY-MM-DD",
    "report_ids":[数字]
  }
}`
	resp, err := a.model.Generate(ctx, []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	})
	if err != nil {
		return llmRouteDecision{}, err
	}
	if resp == nil {
		return llmRouteDecision{}, errors.New("empty_response")
	}
	content := strings.TrimSpace(resp.Content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return llmRouteDecision{}, errors.New("json_not_found")
	}
	content = content[start : end+1]
	var out llmRouteDecision
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return llmRouteDecision{}, err
	}
	return out, nil
}

func normalizeIntent(raw string) IntentType {
	switch IntentType(strings.ToLower(strings.TrimSpace(raw))) {
	case IntentGeneralChat:
		return IntentGeneralChat
	case IntentPortfolio:
		return IntentPortfolio
	case IntentDecision:
		return IntentDecision
	case IntentStockPicker:
		return IntentStockPicker
	default:
		return ""
	}
}

func buildRouteDecision(intent IntentType, confidence float64, reason string, slots RouteSlots, fallback bool) RouteDecision {
	return RouteDecision{
		Intent:       intent,
		Workflow:     workflowForIntent(intent),
		Confidence:   confidence,
		Reason:       strings.TrimSpace(reason),
		Slots:        slots,
		AllowedTools: toolsForIntent(intent),
		Fallback:     fallback,
	}
}

func fallbackRoute(reason string) RouteDecision {
	return RouteDecision{
		Intent:       IntentGeneralChat,
		Workflow:     WorkflowChatGeneral,
		Confidence:   0,
		Reason:       strings.TrimSpace(reason),
		AllowedTools: []string{},
		Fallback:     true,
	}
}

func workflowForIntent(intent IntentType) WorkflowType {
	switch intent {
	case IntentPortfolio:
		return WorkflowChatPortfolio
	case IntentDecision:
		return WorkflowDomainDecision
	case IntentStockPicker:
		return WorkflowDomainPicker
	default:
		return WorkflowChatGeneral
	}
}

func toolsForIntent(intent IntentType) []string {
	switch intent {
	case IntentPortfolio:
		return []string{"investment"}
	default:
		return []string{}
	}
}

func sanitizeRouteSlots(raw map[string]interface{}) RouteSlots {
	if len(raw) == 0 {
		return RouteSlots{}
	}
	var slots RouteSlots
	if v, ok := parseInt64Raw(raw["account_id"]); ok && v > 0 {
		slots.AccountID = v
	}
	if v, ok := parseIntRaw(raw["top_n"]); ok && v > 0 {
		slots.TopN = v
	}
	if v, ok := parseDateRaw(raw["date_from"]); ok {
		slots.DateFrom = v
	}
	if v, ok := parseDateRaw(raw["date_to"]); ok {
		slots.DateTo = v
	}
	slots.ReportIDs = parseReportIDsRaw(raw["report_ids"])
	return slots
}

func parseIntRaw(raw interface{}) (int, bool) {
	v, ok := parseInt64Raw(raw)
	if !ok {
		return 0, false
	}
	if v > int64(^uint(0)>>1) {
		return 0, false
	}
	return int(v), true
}

func parseInt64Raw(raw interface{}) (int64, bool) {
	switch v := raw.(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return n, true
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return 0, false
		}
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func parseDateRaw(raw interface{}) (string, bool) {
	text, ok := raw.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	if _, err := time.Parse("2006-01-02", text); err != nil {
		return "", false
	}
	return text, true
}

func parseReportIDsRaw(raw interface{}) []int64 {
	switch v := raw.(type) {
	case []interface{}:
		out := make([]int64, 0, len(v))
		for _, item := range v {
			if n, ok := parseInt64Raw(item); ok && n > 0 {
				out = append(out, n)
			}
		}
		return clampReportIDs(out)
	case []int64:
		return clampReportIDs(v)
	case string:
		parts := strings.Split(v, ",")
		out := make([]int64, 0, len(parts))
		for _, part := range parts {
			if n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && n > 0 {
				out = append(out, n)
			}
		}
		return clampReportIDs(out)
	default:
		return nil
	}
}

func clampReportIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		out = append(out, id)
		if len(out) >= 20 {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
