package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// LLMServiceAdapter adapts EinoChatModel to LLMService interface
type LLMServiceAdapter struct {
	model model.ToolCallingChatModel
}

// NewLLMServiceAdapter creates a new LLM service adapter
func NewLLMServiceAdapter(m model.ToolCallingChatModel) *LLMServiceAdapter {
	return &LLMServiceAdapter{model: m}
}

// ChatComplete sends a chat completion request
func (a *LLMServiceAdapter) ChatComplete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if a.model == nil {
		return "", fmt.Errorf("model_not_initialized")
	}

	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}

	resp, err := a.model.Generate(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("generating response: %w", err)
	}

	if resp == nil {
		return "", fmt.Errorf("empty_response")
	}

	return resp.Content, nil
}

// ChatCompleteWithHistory sends a chat completion request with message history
func (a *LLMServiceAdapter) ChatCompleteWithHistory(ctx context.Context, msgs []*schema.Message) (string, error) {
	if a.model == nil {
		return "", fmt.Errorf("model_not_initialized")
	}

	resp, err := a.model.Generate(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("generating response: %w", err)
	}

	if resp == nil {
		return "", fmt.Errorf("empty_response")
	}

	return resp.Content, nil
}

// Label implements news.LLMLabelService interface for labeling news
// LabelSet type for interface compatibility
type LabelSet struct {
	Sentiment      float64    `json:"sentiment"`
	Novelty        string     `json:"novelty"`
	Predictability string     `json:"predictability"`
	Timeframe      string     `json:"timeframe"`
	Entities       []string   `json:"entities"`
}

func (a *LLMServiceAdapter) Label(ctx context.Context, title, summary string, domains []EventDomain) (*LabelSet, error) {
	if a.model == nil {
		return nil, fmt.Errorf("model_not_initialized")
	}

	// Build prompt for labeling
	systemPrompt := `你是一个新闻标签分析引擎。请分析新闻的情感、新颖度、可预期性和时间框架。
输出严格 JSON 格式：{"sentiment": -1.0到1.0的数值, "novelty": "breaking|follow_up|recurring|old_news", "predictability": "scheduled|unscheduled|semi_known", "timeframe": "immediate|short|medium|long"}`

	userPrompt := fmt.Sprintf("标题: %s\n摘要: %s\n事件域: %v", title, summary, domains)

	response, err := a.ChatComplete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	labels := &LabelSet{}
	// Extract JSON from response
	if start := strings.Index(response, "{"); start >= 0 {
		if end := strings.LastIndex(response, "}"); end > start {
			jsonStr := response[start : end+1]
			if err := json.Unmarshal([]byte(jsonStr), labels); err != nil {
				return nil, fmt.Errorf("parsing labels: %w", err)
			}
		}
	}

	return labels, nil
}
