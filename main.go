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
	"genFu/internal/access"
	"genFu/internal/agent"
	"genFu/internal/agent/bear"
	"genFu/internal/agent/bull"
	"genFu/internal/agent/debate"
	decisionagent "genFu/internal/agent/decision"
	executionplanner "genFu/internal/agent/execution_planner"
	fundmanager "genFu/internal/agent/fund_manager"
	"genFu/internal/agent/kline"
	pdfsummaryagent "genFu/internal/agent/pdfsummary"
	portfoliofitagent "genFu/internal/agent/portfoliofit"
	posttradereview "genFu/internal/agent/post_trade_review"
	regimeagent "genFu/internal/agent/regime"
	stockpickeragent "genFu/internal/agent/stockpicker"
	stockscreener "genFu/internal/agent/stockscreener"
	"genFu/internal/agent/summary"
	tradeguidecompileragent "genFu/internal/agent/tradeguidecompiler"
	"genFu/internal/analyze"
	"genFu/internal/api"
	"genFu/internal/chat"
	"genFu/internal/config"
	"genFu/internal/conversationlog"
	"genFu/internal/db"
	decision "genFu/internal/decision"
	"genFu/internal/financial"
	"genFu/internal/investment"
	"genFu/internal/llm"
	"genFu/internal/news"
	"genFu/internal/router"
	"genFu/internal/rsshub"
	"genFu/internal/server"
	stockpicker "genFu/internal/stockpicker"
	"genFu/internal/tool"
	"genFu/internal/trade_signal"
	"genFu/internal/tushare"
	"genFu/internal/workflow"
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
	investmentRepo := investment.NewRepository(database)
	investmentSvc := investment.NewService(investmentRepo)
	eastMoneyTool := tool.NewEastMoneyToolWithOptions(tool.EastMoneyOptions{
		Timeout:     appConfig.EastMoney.Timeout,
		MaxRetries:  appConfig.EastMoney.MaxRetries,
		MinInterval: appConfig.EastMoney.MinInterval,
		Referer:     appConfig.EastMoney.Referer,
		UserAgent:   appConfig.EastMoney.UserAgent,
	})

	// --- Tushare Pro ---
	var tushareTool *tool.TushareTool
	if appConfig.Tushare.Token != "" {
		tsCfg := tushare.Config{
			Token:      appConfig.Tushare.Token,
			BaseURL:    appConfig.Tushare.BaseURL,
			Timeout:    appConfig.Tushare.Timeout,
			MaxRetries: appConfig.Tushare.MaxRetries,
			RateLimit:  appConfig.Tushare.RateLimit,
		}
		tsClient := tushare.NewClient(tsCfg)
		tushareTool = tool.NewTushareTool(tsClient)
	}

	registry.Register(tool.NewInvestmentToolWithEastMoney(investmentSvc, eastMoneyTool))
	registry.Register(eastMoneyTool)
	registry.Register(tool.NewMarketDataTool(investmentSvc))
	registry.Register(tool.NewCNInfoTool())
	if tushareTool != nil {
		registry.Register(tushareTool)
		log.Printf("Tushare Tool 已注册")
	}
	newsRepo := news.NewRepository(database)
	registry.Register(tool.NewBriefSearchTool(newsRepo))
	analyzeRepo := analyze.NewRepository(database)

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
