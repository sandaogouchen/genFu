package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

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
	"genFu/internal/ruleengine"
	"genFu/internal/ruleengine/strategies"
	"genFu/internal/server"
	stockpicker "genFu/internal/stockpicker"
	"genFu/internal/tool"
	"genFu/internal/trade_signal"
	"genFu/internal/tushare"
	"genFu/internal/workflow"
)

func main() {
	registry := tool.NewRegistry()
	registry.Register(tool.EchoTool{})

	appConfig, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	dbConfig := db.Config{
		DSN:             appConfig.PG.DSN,
		MaxOpenConns:    appConfig.PG.MaxOpenConns,
		MaxIdleConns:    appConfig.PG.MaxIdleConns,
		ConnMaxLifetime: appConfig.PG.ConnMaxLifetime,
	}
	database, err := db.Open(dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err := database.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}
	if err := db.ApplyMigrations(context.Background(), database); err != nil {
		log.Fatal(err)
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

	log.Printf("使用直连LLM endpoint=%s model=%s", appConfig.LLM.Endpoint, appConfig.LLM.Model)
	chatModel, err := llm.NewEinoChatModel(appConfig.LLM)
	if err != nil {
		log.Fatal(err)
	}
	chatRepo := chat.NewRepository(database)
	var chatService *chat.Service
	conversationRepo := conversationlog.NewRepository(database)

	defaultAgent := agent.NewFuncAgent("default", []string{"chat"}, agent.DefaultGenerate)
	toolAgent := agent.NewFuncAgent("tool", []string{"tool"}, agent.ToolGenerate)
	fundManagerAgent, err := fundmanager.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	bullAgent, err := bull.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	bearAgent, err := bear.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	debateAgent, err := debate.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	klineAgent, err := kline.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	summaryAgent, err := summary.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	decisionAgent, err := decisionagent.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	executionPlannerAgent, err := executionplanner.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	postTradeReviewAgent, err := posttradereview.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	pdfSummaryAgent, err := pdfsummaryagent.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	stockpickerAgent, err := stockpickeragent.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	regimeAgent, err := regimeagent.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	portfolioFitAgent, err := portfoliofitagent.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	tradeGuideCompilerAgent, err := tradeguidecompileragent.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}
	// 创建筛选Agent
	stockScreenerAgent, err := stockscreener.New(chatModel, registry)
	if err != nil {
		log.Fatal(err)
	}

	// 注册股票筛选工具
	registry.Register(stockpicker.NewStockScreenerTool(registry))
	stockpicker.RegisterStockStrategyTools(registry)

	analyzer := analyze.NewAnalyzer(klineAgent, fundManagerAgent, bullAgent, bearAgent, debateAgent, summaryAgent, registry, analyzeRepo)
	tradeEngine := trade_signal.NewInvestmentEngine(investmentSvc)
	newsProvider := decision.NewBriefNewsProvider(newsRepo, investmentRepo, appConfig.News.AccountID, appConfig.News.BriefLimit, appConfig.News.Keywords)
	decisionRepo := decision.NewRepository(database)
	financialRepo := financial.NewRepository(database)
	pdfAgentAdapter := NewPDFAgentAdapter(pdfSummaryAgent)
	financialSvc := financial.NewService(financialRepo, pdfAgentAdapter)
	stockpickerProvider := stockpicker.NewDataProvider(newsRepo, investmentRepo, analyzeRepo, registry, financialSvc)
	stockpickerGuideRepo := stockpicker.NewGuideRepository(database)
	stockpickerRunRepo := stockpicker.NewRunRepository(database)
	decisionSvc := decision.NewService(
		decisionAgent,
		registry,
		tradeEngine,
		analyzeRepo,
		investmentRepo,
		stockpickerGuideRepo,
		newsProvider,
		decision.WithExecutionPlannerAgent(executionPlannerAgent),
		decision.WithPostTradeReviewAgent(postTradeReviewAgent),
		decision.WithPolicyGuard(decision.NewPolicyGuardAgent(investmentRepo)),
		decision.WithDecisionRepository(decisionRepo),
		decision.WithRiskBudgetDefaults(decision.RiskBudget{
			MaxSingleOrderRatio:    appConfig.Decision.MaxSingleOrderRatio,
			MaxSymbolExposureRatio: appConfig.Decision.MaxSymbolExposureRatio,
			MaxDailyTradeRatio:     appConfig.Decision.MaxDailyTradeRatio,
			MinConfidence:          appConfig.Decision.MinConfidence,
		}),
	)
	stockpickerSvc := stockpicker.NewService(
		regimeAgent,
		stockScreenerAgent,
		stockpickerAgent,
		portfolioFitAgent,
		tradeGuideCompilerAgent,
		registry,
		stockpickerProvider,
		stockpickerGuideRepo,
		stockpickerRunRepo,
	)
	intentRouter := chat.NewIntentRouterAgent(chatModel)
	sessionMemoryAgent := chat.NewSessionMemoryAgent(chatModel)
	chatService = chat.NewService(
		chatModel,
		chatRepo,
		registry,
		chat.WithDecisionService(decisionSvc),
		chat.WithStockPickerService(stockpickerSvc),
		chat.WithIntentRouter(intentRouter),
		chat.WithSessionMemoryAgent(sessionMemoryAgent),
	)
	dailyReviewSvc := analyze.NewDailyReviewService(chatModel, registry, analyzeRepo, investmentRepo)
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	dailyReviewScheduler := analyze.NewDailyReviewScheduler(dailyReviewSvc, 15, 30, loc)
	dailyReviewScheduler.Start(context.Background())
	rsshubClient := rsshub.NewClient(appConfig.RSSHub.BaseURL, appConfig.RSSHub.Timeout)
	registry.Register(tool.NewRSSHubTool(appConfig.RSSHub.BaseURL, appConfig.RSSHub.Routes, appConfig.RSSHub.Timeout))
	nextOpenSvc := analyze.NewNextOpenGuideService(chatModel, registry, analyzeRepo, investmentRepo, appConfig.NextOpen.AccountID, appConfig.RSSHub.Routes, appConfig.NextOpen.NewsLimit)
	nextOpenScheduler := analyze.NewNextOpenGuideScheduler(nextOpenSvc, appConfig.NextOpen.Hour, appConfig.NextOpen.Minute, loc)
	if appConfig.NextOpen.Enabled {
		nextOpenScheduler.Start(context.Background())
	}
	// 旧系统 - 已被新pipeline替代
	// newsGenerator := newsgen.NewLLMGenerator(chatModel)
	// newsService := news.NewService(newsRepo, rsshubClient, newsGenerator, appConfig.RSSHub.Routes, appConfig.RSSHub.MaxItems)
	// newsScheduler := news.NewScheduler(newsService, appConfig.RSSHub.PollInterval)
	// newsScheduler.Start(context.Background())

	// Initialize news analysis pipeline
	var newsPipeline *news.Pipeline
	if appConfig.News.PipelineEnabled {
		// Create embedding service (optional - will use fallback if not configured)
		var embedSvc *llm.EmbeddingService
		if appConfig.Embedding.APIKey != "" {
			embedSvc = llm.NewEmbeddingService(llm.EmbeddingConfig{
				Provider: appConfig.Embedding.Provider,
				APIKey:   appConfig.Embedding.APIKey,
				Model:    appConfig.Embedding.Model,
				BaseURL:  appConfig.Embedding.BaseURL,
				Timeout:  appConfig.Embedding.Timeout,
			})
			log.Printf("Embedding服务已启用: provider=%s model=%s", appConfig.Embedding.Provider, appConfig.Embedding.Model)
		} else {
			log.Printf("警告: 未配置Embedding API Key，将使用降级模式（跳过语义分析和去重）")
		}

		// Create LLM service adapter
		llmAdapter := llm.NewLLMServiceAdapter(chatModel)

		// Build portfolio context
		portfolioContext, err := news.BuildPortfolioContext(context.Background(), investmentRepo)
		if err != nil {
			log.Printf("警告: 构建投资组合上下文失败: %v", err)
			portfolioContext = &news.PortfolioContext{}
		}

		// Create news analysis components
		rssHubSource := news.NewRSSHubSource(rsshubClient, appConfig.RSSHub.Routes, appConfig.RSSHub.MaxItems)
		newsCollector := news.NewCollector(embedSvc, rssHubSource)

		// Create adapters for interface compatibility (only if embedding is available)
		var newsTagger *news.Tagger
		if embedSvc != nil {
			embedClassifierAdapter := NewEmbeddingClassifierAdapter(embedSvc)
			llmLabelAdapter := NewLLMLabelServiceAdapter(llmAdapter)
			newsTagger = news.NewTagger(
				news.WithEmbeddingClassifier(embedClassifierAdapter),
				news.WithLLMLabelService(llmLabelAdapter),
			)
		} else {
			// Fallback: only use keyword rules and LLM labeling
			llmLabelAdapter := NewLLMLabelServiceAdapter(llmAdapter)
			newsTagger = news.NewTagger(
				news.WithLLMLabelService(llmLabelAdapter),
			)
		}

		funnelConfig := news.DefaultConfig()
		funnelConfig.EventImpactEnabled = appConfig.News.Pipeline.EventImpactEnabled
		funnelConfig.CausalVerifierEnabled = appConfig.News.Pipeline.CausalVerifierEnabled
		funnelConfig.EventImpactBatchSize = appConfig.News.Pipeline.EventImpactBatchSize
		funnelConfig.VerifierMaxAnalyze = appConfig.News.Pipeline.VerifierMaxAnalyze
		funnelConfig.VerifierWeakThreshold = appConfig.News.Pipeline.VerifierWeakThreshold
		funnelConfig.VerifierInvalidThreshold = appConfig.News.Pipeline.VerifierInvalidThreshold

		var eventImpactAgent news.EventImpactAgent
		if appConfig.News.Pipeline.EventImpactEnabled {
			eventImpactAgent = news.NewLLMEventImpactAgent(llmAdapter, appConfig.News.Pipeline.EventImpactBatchSize)
		}
		var causalVerifierAgent news.CausalVerifierAgent
		if appConfig.News.Pipeline.CausalVerifierEnabled {
			causalVerifierAgent = news.NewLLMCausalVerifierAgent(
				llmAdapter,
				appConfig.News.Pipeline.VerifierWeakThreshold,
				appConfig.News.Pipeline.VerifierInvalidThreshold,
			)
		}

		newsFunnel := news.NewFunnel(
			embedSvc,
			llmAdapter,
			portfolioContext,
			news.WithFunnelConfig(funnelConfig),
			news.WithEventImpactAgent(eventImpactAgent),
			news.WithCausalVerifierAgent(causalVerifierAgent),
		)

		// Create news pipeline
		newsPipeline = news.NewPipeline(
			newsCollector, newsTagger, newsFunnel, portfolioContext, newsRepo,
			news.WithPipelineConfig(news.PipelineConfig{
				PreMarketTime:    appConfig.News.Pipeline.PreMarketTime,
				IntradayInterval: appConfig.News.Pipeline.IntradayInterval,
				TradingStart:     appConfig.News.Pipeline.TradingStart,
				TradingEnd:       appConfig.News.Pipeline.TradingEnd,
				LookbackDuration: appConfig.News.Pipeline.LookbackDuration,
			}),
		)

		// Start scheduled dispatching
		if err := newsPipeline.Start(context.Background()); err != nil {
			log.Printf("警告: 启动新闻流水线失败: %v", err)
		} else {
			log.Printf("新闻分析流水线已启动")
		}
	}

	// Rule Engine initialization (if enabled)
	if appConfig.RuleEngine.Enabled {
		ruleStore := ruleengine.NewSQLiteRuleStore(database)
		posTracker := ruleengine.NewSQLitePositionTracker(database)

		reStrategies := []ruleengine.StrategyEvaluator{
			&strategies.FixedPctSL{},
			&strategies.TrailingSL{},
			&strategies.ATRSL{},
			&strategies.DailyDropSL{},
			&strategies.FixedPctTP{},
			&strategies.TrailingTP{},
			&strategies.PartialTP{},
		}

		engine := ruleengine.NewEngine(ruleStore, posTracker, reStrategies)

		// Register RuleEngine tool
		reTool := tool.NewRuleEngineTool(engine, ruleStore)
		_ = reTool // TODO: register with tool registry

		// Start monitor
		monitor := ruleengine.NewMonitor(engine, posTracker, ruleStore,
			ruleengine.WithAccountID(0), // default account
		)
		go monitor.Start(context.Background())
		defer monitor.Stop()

		log.Println("[RuleEngine] Started with", len(reStrategies), "strategies")
	}

	stockWF, err := workflow.NewStockWorkflow(context.Background(), chatModel, registry, investmentRepo, appConfig.RSSHub.Routes)
	if err != nil {
		log.Fatal(err)
	}

	r := router.NewRouter(defaultAgent)
	r.AddRoute([]string{"tool:", "tool:eastmoney", "call:echo", "call:investment", "call:eastmoney", "investment", "投资", "持仓"}, toolAgent)
	r.AddRoute([]string{"基金经理", "基金编号", "经理分析", "manager"}, fundManagerAgent)
	r.AddRoute([]string{"多头", "看涨", "bull"}, bullAgent)
	r.AddRoute([]string{"空头", "看跌", "bear"}, bearAgent)
	r.AddRoute([]string{"辩论", "多空", "debate"}, debateAgent)

	ocrHandler := api.NewOcrHoldingsHandler(appConfig.LLM, investmentSvc, "internal/agent/prompt/ocr_holdings.md")
	srv := server.NewServer(r, registry, analyzer, decisionSvc, stockpickerSvc, stockpickerGuideRepo, chatService, stockWF, ocrHandler, newsPipeline, newsRepo, conversationRepo)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	addr := ":" + strconv.Itoa(appConfig.Server.Port)

	log.Printf("server listening on %s", addr)
	var handler http.Handler = mux
	if appConfig.Access.Enabled {
		manager := access.NewManager(appConfig.Access.APIKeys)
		handler = access.WrapHTTP(mux, manager, appConfig.Access.AllowPaths)
	}
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
