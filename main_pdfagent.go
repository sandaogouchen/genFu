package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"genFu/internal/agent"
	"genFu/internal/financial"
	"genFu/internal/generate"
	"genFu/internal/message"
)

// PDFAgentAdapter 适配agent.Agent到financial.PDFAgent接口
type PDFAgentAdapter struct {
	agent  agent.Agent
	client *financial.CNInfoClient
}

func NewPDFAgentAdapter(agent agent.Agent) *PDFAgentAdapter {
	return &PDFAgentAdapter{
		agent:  agent,
		client: financial.NewCNInfoClient(),
	}
}

func (a *PDFAgentAdapter) AnalyzePDF(ctx context.Context, ann *financial.Announcement) (*financial.ReportSummary, error) {
	// 获取PDF下载URL
	pdfURL := a.client.GetPDFDownloadURL(ann)

	// 构建给Agent的提示
	prompt := fmt.Sprintf(`请分析以下财报PDF文件：

股票代码: %s
公司名称: %s
公告标题: %s
PDF链接: %s

请使用 cninfo 工具的 download_pdf action 下载并分析这份财报，提取关键信息。`,
		ann.SecCode, ann.SecName, ann.Title, pdfURL)

	// 调用Agent
	resp, err := a.agent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{
			{Role: message.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		return nil, err
	}

	// 解析结果
	var summary financial.ReportSummary
	content := strings.TrimSpace(resp.Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	if err := json.Unmarshal([]byte(content), &summary); err != nil {
		return nil, fmt.Errorf("parse agent output: %w", err)
	}

	// 补充信息
	summary.Symbol = ann.SecCode
	summary.CompanyName = ann.SecName
	summary.ReportTitle = ann.Title
	summary.GeneratedAt = time.Now()

	return &summary, nil
}
