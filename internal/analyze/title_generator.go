package analyze

import (
	"context"
	"fmt"
	"strings"

	"genFu/internal/agent"
	"genFu/internal/generate"
	"genFu/internal/message"
)

// TitleGenerator generates concise titles for reports using LLM
type TitleGenerator struct {
	agent agent.Agent
	repo  *Repository
}

// NewTitleGenerator creates a new title generator
func NewTitleGenerator(agent agent.Agent, repo *Repository) *TitleGenerator {
	return &TitleGenerator{
		agent: agent,
		repo:  repo,
	}
}

// GenerateTitle generates a concise title (<=20 chars) from report summary
func (tg *TitleGenerator) GenerateTitle(ctx context.Context, summary string) (string, error) {
	if tg.agent == nil {
		return "", fmt.Errorf("title_generator_agent_not_initialized")
	}

	// Truncate summary if too long to save tokens
	maxSummaryLen := 500
	if len(summary) > maxSummaryLen {
		summary = summary[:maxSummaryLen]
	}

	prompt := fmt.Sprintf(`请为以下分析报告生成一个简洁的标题（不超过20个汉字）。

报告摘要：
%s

要求：
1. 标题要突出关键观点或结论
2. 控制在20个汉字以内
3. 不要包含标点符号
4. 直接输出标题，不要其他内容`, summary)

	req := generate.GenerateRequest{
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: prompt,
			},
		},
	}

	resp, err := tg.agent.Handle(ctx, req)
	if err != nil {
		return "", fmt.Errorf("title_generation_failed: %w", err)
	}

	title := strings.TrimSpace(resp.Message.Content)
	// Remove quotes if present
	title = strings.Trim(title, `"'"`)
	// Limit to 20 Chinese characters (approximately 60 bytes for UTF-8)
	if len(title) > 60 {
		title = title[:60]
	}

	return title, nil
}

// GenerateAndSave generates a title and updates the report
func (tg *TitleGenerator) GenerateAndSave(ctx context.Context, reportID int64, summary string) error {
	title, err := tg.GenerateTitle(ctx, summary)
	if err != nil {
		return err
	}

	return tg.repo.UpdateReportTitle(ctx, reportID, title)
}
