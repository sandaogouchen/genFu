package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"genFu/internal/agent"
	"genFu/internal/analyze"
	"genFu/internal/generate"
	"genFu/internal/investment"
	"genFu/internal/message"
	"genFu/internal/tool"
	"genFu/internal/trade_signal"
)

type Service struct {
	agent    agent.Agent
	registry *tool.Registry
	engine   trade_signal.Engine
	reports  *analyze.Repository
	holdings *investment.Repository
	provider MarketNewsProvider
}

func NewService(agent agent.Agent, registry *tool.Registry, engine trade_signal.Engine, reports *analyze.Repository, holdings *investment.Repository, provider MarketNewsProvider) *Service {
	if provider == nil {
		provider = EmptyMarketNewsProvider{}
	}
	return &Service{agent: agent, registry: registry, engine: engine, reports: reports, holdings: holdings, provider: provider}
}

func (s *Service) Decide(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	if s == nil {
		return DecisionResponse{}, errors.New("decision_service_not_initialized")
	}

	println("\n" + strings.Repeat("=", 80))
	println("[DECISION SERVICE] 开始交易决策流程")
	println(strings.Repeat("=", 80))

	// 1. 加载分析报告
	println("\n[DECISION SERVICE] 步骤 1: 加载分析报告")
	reportTexts, err := s.loadReports(ctx, req.ReportIDs)
	if err != nil {
		printf("[DECISION SERVICE] ✗ 加载报告失败: %v\n", err)
		return DecisionResponse{}, err
	}
	printf("[DECISION SERVICE] ✓ 成功加载 %d 份报告\n", len(reportTexts))
	for i, report := range reportTexts {
		printf("  报告 %d 长度: %d 字符\n", i+1, len(report))
	}

	// 2. 确定账户ID
	println("\n[DECISION SERVICE] 步骤 2: 确定账户ID")
	accountID := req.AccountID
	printf("  请求中的账户ID: %d\n", accountID)
	if accountID == 0 && s.holdings != nil {
		accountID, err = s.holdings.DefaultAccountID(ctx)
		if err != nil {
			printf("[DECISION SERVICE] ✗ 获取默认账户失败: %v\n", err)
			return DecisionResponse{}, err
		}
		printf("  使用默认账户ID: %d\n", accountID)
	}

	// 3. 加载持仓信息
	println("\n[DECISION SERVICE] 步骤 3: 加载持仓信息")
	holdingsText, err := s.loadHoldings(ctx, accountID)
	if err != nil {
		printf("[DECISION SERVICE] ✗ 加载持仓失败: %v\n", err)
		return DecisionResponse{}, err
	}
	printf("[DECISION SERVICE] ✓ 持仓数据长度: %d 字符\n", len(holdingsText))
	if holdingsText != "" {
		printf("  持仓数据预览: %s...\n", truncateString(holdingsText, 200))
	}

	// 4. 加载市场新闻
	println("\n[DECISION SERVICE] 步骤 4: 加载市场和新闻数据")
	marketText, newsText := s.loadMarketNews(ctx)
	printf("  市场数据: %d 字符\n", len(marketText))
	if marketText != "" {
		printf("  市场数据预览: %s...\n", truncateString(marketText, 200))
	}
	printf("  新闻数据: %d 字符\n", len(newsText))
	if newsText != "" {
		printf("  新闻数据预览: %s...\n", truncateString(newsText, 200))
	}

	// 5. 构建决策输入
	println("\n[DECISION SERVICE] 步骤 5: 构建决策输入")
	input := buildDecisionInput(req, holdingsText, marketText, newsText, reportTexts)
	printf("  输入长度: %d 字符\n", len(input))
	printf("  输入预览:\n%s\n", truncateString(input, 500))

	// 6. 调用Agent处理
	println("\n[DECISION SERVICE] 步骤 6: 调用决策Agent")
	printf("  Agent名称: %s\n", s.agent.Name())
	printf("  Agent能力: %v\n", s.agent.Capabilities())

	// 检查注册的工具
	if s.registry != nil {
		tools := s.registry.List()
		printf("  可用工具数量: %d\n", len(tools))
		for i, spec := range tools {
			printf("    工具 %d: %s - %s\n", i+1, spec.Name, spec.Description)
		}
	} else {
		println("  ⚠ 工具注册表为空")
	}

	resp, err := s.agent.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{
			{Role: message.RoleUser, Content: input},
		},
	})
	if err != nil {
		printf("[DECISION SERVICE] ✗ Agent处理失败: %v\n", err)
		return DecisionResponse{}, err
	}

	println("\n[DECISION SERVICE] ✓ Agent处理完成")
	printf("  响应消息长度: %d 字符\n", len(resp.Message.Content))
	printf("  响应预览:\n%s\n", truncateString(resp.Message.Content, 500))
	printf("  工具调用次数: %d\n", len(resp.ToolCalls))

	// 7. 解析工具结果
	toolResults := parseToolResultsMeta(resp.Meta)
	printf("  工具结果数量: %d\n", len(toolResults))
	for i, result := range toolResults {
		printf("    结果 %d: 工具=%s, 错误=%s\n", i+1, result.Name, result.Error)
	}

	// 8. 解析决策输出
	println("\n[DECISION SERVICE] 步骤 7: 解析决策输出")
	decision, signals, err := trade_signal.ParseDecisionOutput(resp.Message.Content, accountID)
	if err != nil {
		printf("[DECISION SERVICE] ✗ 解析决策输出失败: %v\n", err)
		return DecisionResponse{}, err
	}

	printf("[DECISION SERVICE] ✓ 决策ID: %s\n", decision.DecisionID)
	printf("  市场观点: %s\n", truncateString(decision.MarketView, 100))
	printf("  风险提示: %s\n", truncateString(decision.RiskNotes, 100))
	printf("  交易信号数量: %d\n", len(signals))

	// 9. 执行交易信号
	executions := []trade_signal.ExecutionResult{}
	if s.engine != nil {
		println("\n[DECISION SERVICE] 步骤 8: 执行交易信号")
		executions, err = s.engine.Execute(ctx, signals)
		if err != nil {
			printf("[DECISION SERVICE] ✗ 执行交易信号失败: %v\n", err)
			return DecisionResponse{}, err
		}
		printf("[DECISION SERVICE] ✓ 执行结果数量: %d\n", len(executions))
	}

	println("\n" + strings.Repeat("=", 80))
	println("[DECISION SERVICE] 交易决策流程完成")
	println(strings.Repeat("=", 80) + "\n")

	return DecisionResponse{
		Decision:    decision,
		Raw:         resp.Message.Content,
		Signals:     signals,
		Executions:  executions,
		ToolResults: toolResults,
	}, nil
}

func parseToolResultsMeta(meta map[string]string) []tool.ToolResult {
	if meta == nil {
		return nil
	}
	raw := strings.TrimSpace(meta["tool_results"])
	if raw == "" {
		return nil
	}
	results := []tool.ToolResult{}
	if err := json.Unmarshal([]byte(raw), &results); err != nil {
		return nil
	}
	return results
}

func buildDecisionInput(req DecisionRequest, holdings string, market string, news string, reports []string) string {
	payloadMap := map[string]interface{}{
		"holdings":       holdings,
		"market_summary": market,
		"news_summary":   news,
		"reports":        reports,
		"meta":           req.Meta,
	}
	if strings.TrimSpace(news) == "" {
		delete(payloadMap, "news_summary")
	}
	payload, _ := json.Marshal(payloadMap)
	return "生成交易决策，严格输出JSON：\n" + strings.TrimSpace(string(payload))
}

func (s *Service) loadReports(ctx context.Context, ids []int64) ([]string, error) {
	if s.reports == nil || len(ids) == 0 {
		return nil, nil
	}
	results := make([]string, 0, len(ids))
	for _, id := range ids {
		report, err := s.reports.GetReport(ctx, id)
		if err != nil {
			return nil, err
		}
		results = append(results, report.Summary)
	}
	return results, nil
}

func (s *Service) loadHoldings(ctx context.Context, accountID int64) (string, error) {
	if s.holdings == nil || accountID == 0 {
		return "", nil
	}
	positions, err := s.holdings.ListPositions(ctx, accountID)
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(positions)
	return string(payload), nil
}

func (s *Service) loadMarketNews(ctx context.Context) (string, string) {
	if s.provider == nil {
		return "", ""
	}
	market, err := s.provider.GetMarketSummary(ctx)
	if err != nil {
		market = ""
	}
	news, err := s.provider.GetNewsSummary(ctx)
	if err != nil {
		news = ""
	}
	return market, news
}

// 辅助函数
func printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
