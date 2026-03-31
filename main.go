package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/sandaogouchen/genFu/internal/agent"
	"github.com/sandaogouchen/genFu/internal/agent/technical"
	"github.com/sandaogouchen/genFu/internal/analyze"
	"github.com/sandaogouchen/genFu/internal/tool"
)

func main() {
	// 从环境变量读取配置
	apiKey := os.Getenv("LLM_API_KEY")
	apiURL := os.Getenv("LLM_API_URL")
	model := os.Getenv("LLM_MODEL")
	marketDataURL := os.Getenv("MARKET_DATA_URL")

	if apiKey == "" {
		log.Fatal("LLM_API_KEY environment variable is required")
	}
	if apiURL == "" {
		apiURL = "https://api.openai.com"
	}
	if model == "" {
		model = "gpt-4"
	}
	if marketDataURL == "" {
		marketDataURL = "https://api.binance.com"
	}

	// 注册工具
	registry := tool.GetRegistry()
	registry.Register(tool.NewMarketDataTool(marketDataURL))
	registry.Register(tool.NewIndicatorTool(registry)) // 新增：注册技术指标工具

	// 创建Agent
	llmAgent := agent.NewLLMAgent(agent.LLMConfig{
		APIKey:     apiKey,
		APIURL:     apiURL,
		Model:      model,
		MaxRetries: 5,
	})

	// 创建技术分析 Agent（新增）
	techAgent := technical.New()

	// 创建分析服务（使用增强版构造函数）
	svc := analyze.NewServiceWithTechnical(llmAgent, techAgent)

	// 执行分析
	ctx := context.Background()
	resp, err := svc.Analyze(ctx, analyze.AnalyzeRequest{
		Symbol:     "BTCUSDT",
		Period:     "1d",
		Indicators: []string{"MA", "RSI", "MACD", "布林带"},
	})
	if err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}

	fmt.Printf("=== Analysis for %s (%s) ===\n", resp.Symbol, resp.Period)
	fmt.Println(resp.Analysis)

	// 输出技术分析结果（如有）
	if resp.TechnicalAnalysis != "" {
		fmt.Println("\n=== Technical Analysis ===")
		fmt.Println(resp.TechnicalAnalysis)
	}
}
