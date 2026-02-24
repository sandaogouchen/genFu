package newsgen

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/news"
)

type LLMGenerator struct {
	model model.ToolCallingChatModel
}

func NewLLMGenerator(model model.ToolCallingChatModel) *LLMGenerator {
	return &LLMGenerator{model: model}
}

func (g *LLMGenerator) Generate(ctx context.Context, item news.NewsItem) (string, string, []string, error) {
	if g == nil || g.model == nil {
		return "", "", nil, errors.New("llm_client_not_initialized")
	}
	payload := map[string]interface{}{
		"title":   item.Title,
		"link":    item.Link,
		"content": item.Content,
		"time":    formatTime(item.PublishedAt),
	}
	raw, _ := json.Marshal(payload)
	resp, err := g.model.Generate(ctx, []*schema.Message{
		schema.SystemMessage(newsSentimentPrompt),
		schema.UserMessage(string(raw)),
	})
	if err != nil {
		return "", "", nil, err
	}
	if resp == nil {
		return "", "", nil, errors.New("empty_llm_response")
	}
	sentiment, brief, keywords := parseSentiment(resp.Content)
	if sentiment == "" || brief == "" {
		return "", "", nil, errors.New("empty_brief")
	}
	return sentiment, brief, keywords, nil
}

func parseSentiment(text string) (string, string, []string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", nil
	}
	var parsed struct {
		Sentiment string   `json:"sentiment"`
		Brief     string   `json:"brief"`
		Keywords  []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		return strings.TrimSpace(parsed.Sentiment), strings.TrimSpace(parsed.Brief), parsed.Keywords
	}
	return "中性", text, nil
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

var newsSentimentPrompt = `你是财经新闻情绪分类助手，请根据输入内容输出严格 JSON：{"sentiment":"利好|利空|中性","brief":"简报摘要","keywords":["关键词1","关键词2"]}。不得输出多余文本。`
