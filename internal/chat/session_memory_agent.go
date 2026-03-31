package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/message"
)

type MemorySummarizer interface {
	Summarize(ctx context.Context, previousSummary string, transcript []message.Message) (string, error)
}

type SessionMemoryAgent struct {
	model model.ToolCallingChatModel
}

func NewSessionMemoryAgent(m model.ToolCallingChatModel) *SessionMemoryAgent {
	return &SessionMemoryAgent{model: m}
}

func (a *SessionMemoryAgent) Summarize(ctx context.Context, previousSummary string, transcript []message.Message) (string, error) {
	if a == nil || a.model == nil {
		return "", errors.New("model_not_initialized")
	}
	payload := map[string]interface{}{
		"previous_summary": strings.TrimSpace(previousSummary),
		"transcript":       compactTranscript(transcript),
	}
	raw, _ := json.Marshal(payload)
	systemPrompt := `你是会话记忆压缩助手。请根据已有摘要和本轮对话，更新会话摘要。
要求：
1) 只输出严格JSON：{"summary":"..."}
2) 使用中文，长度不超过400字
3) 保留用户目标、关键约束、未完成事项
4) 不要输出Markdown，不要补充额外字段`
	resp, err := a.model.Generate(ctx, []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(string(raw)),
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", errors.New("empty_response")
	}
	summary, err := parseSummaryJSON(resp.Content)
	if err != nil {
		return "", err
	}
	return truncateRunes(summary, 400), nil
}

func compactTranscript(items []message.Message) []map[string]string {
	out := make([]map[string]string, 0, len(items))
	for _, item := range items {
		role := strings.TrimSpace(string(item.Role))
		content := strings.TrimSpace(item.Content)
		if role == "" || content == "" {
			continue
		}
		out = append(out, map[string]string{
			"role":    role,
			"content": truncateRunes(content, 300),
		})
	}
	return out
}

func parseSummaryJSON(content string) (string, error) {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return "", errors.New("json_not_found")
	}
	content = content[start : end+1]
	var parsed struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return "", err
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		return "", fmt.Errorf("empty_summary")
	}
	return summary, nil
}

func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if utf8.RuneCountInString(input) <= limit {
		return input
	}
	rs := []rune(input)
	return string(rs[:limit])
}
