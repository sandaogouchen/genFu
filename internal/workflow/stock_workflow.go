package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/agent"
	"genFu/internal/agent/bear"
	"genFu/internal/agent/bull"
	"genFu/internal/agent/debate"
	"genFu/internal/agent/summary"
	"genFu/internal/generate"
	"genFu/internal/investment"
	"genFu/internal/message"
	"genFu/internal/rsshub"
	"genFu/internal/tool"
)

type StockWorkflow struct {
	runner compose.Runnable[StockWorkflowInput, StockWorkflowOutput]
}

func NewStockWorkflow(ctx context.Context, model model.ToolCallingChatModel, registry *tool.Registry, investRepo *investment.Repository) (*StockWorkflow, error) {
	bullAgent, err := bull.New(model, registry)
	if err != nil {
		return nil, err
	}
	bearAgent, err := bear.New(model, registry)
	if err != nil {
		return nil, err
	}
	debateAgent, err := debate.New(model, registry)
	if err != nil {
		return nil, err
	}
	summaryAgent, err := summary.New(model, registry)
	if err != nil {
		return nil, err
	}
	return newStockWorkflowWithAgents(ctx, model, registry, investRepo, bullAgent, bearAgent, debateAgent, summaryAgent)
}

func newStockWorkflowWithAgents(ctx context.Context, model model.ToolCallingChatModel, registry *tool.Registry, investRepo *investment.Repository, bullAgent agent.Agent, bearAgent agent.Agent, debateAgent agent.Agent, summaryAgent agent.Agent) (*StockWorkflow, error) {
	if model == nil {
		return nil, errors.New("model_not_initialized")
	}
	wf := compose.NewWorkflow[StockWorkflowInput, StockWorkflowOutput]()

	holdingsNode := wf.AddLambdaNode("holdings", compose.InvokableLambda(func(ctx context.Context, input holdingsInput) (HoldingsOutput, error) {
		return buildHoldings(ctx, investRepo, input.AccountID)
	}))
	holdingsNode.AddInput(compose.START, compose.MapFields("AccountID", "AccountID"))

	holdingsMarketNode := wf.AddLambdaNode("holdings_market", compose.InvokableLambda(func(ctx context.Context, input holdingsMarketInput) (holdingsMarketOutput, error) {
		return buildHoldingsMarket(ctx, registry, input)
	}))
	holdingsMarketNode.AddInput("holdings", compose.MapFields("Positions", "Positions"))

	targetMarketNode := wf.AddLambdaNode("target_market", compose.InvokableLambda(func(ctx context.Context, input marketInput) (targetMarketOutput, error) {
		return buildTargetMarket(ctx, registry, input)
	}))
	targetMarketNode.AddInput(compose.START, compose.MapFields("Symbol", "Symbol"), compose.MapFields("Name", "Name"))

	newsFetchNode := wf.AddLambdaNode("news_fetch", compose.InvokableLambda(func(ctx context.Context, input newsInput) (newsFetchOutput, error) {
		return fetchNews(ctx, registry, input)
	}))
	newsFetchNode.AddInput(compose.START,
		compose.MapFields("Symbol", "Symbol"),
		compose.MapFields("Name", "Name"),
		compose.MapFields("StockNewsRoutes", "StockNewsRoutes"),
		compose.MapFields("IndustryNewsRoutes", "IndustryNewsRoutes"),
		compose.MapFields("NewsLimit", "NewsLimit"),
	)

	newsSummaryNode := wf.AddLambdaNode("news_summary", compose.InvokableLambda(func(ctx context.Context, input newsSummaryInput) (NewsSummaryOutput, error) {
		return summarizeNews(ctx, model, input.Items)
	}))
	newsSummaryNode.AddInput("news_fetch", compose.MapFields("Items", "Items"))

	bullNode := wf.AddLambdaNode("bull", compose.InvokableLambda(func(ctx context.Context, input agentContext) (agentOutput, error) {
		return runAgent(ctx, bullAgent, input)
	}))
	bullNode.AddInput(compose.START, compose.MapFields("Symbol", "Symbol"), compose.MapFields("Name", "Name"))
	bullNode.AddInput("holdings", compose.MapFields("Positions", "HoldingsPositions"), compose.MapFields("TotalValue", "HoldingsTotalValue"))
	bullNode.AddInput("holdings_market", compose.MapFields("Quotes", "HoldingsMarket"))
	bullNode.AddInput("target_market", compose.MapFields("Quote", "TargetMarket"))
	bullNode.AddInput("news_summary", compose.MapFields("Summary", "NewsSummary"), compose.MapFields("Sentiment", "NewsSentiment"), compose.MapFields("Items", "NewsItems"))

	bearNode := wf.AddLambdaNode("bear", compose.InvokableLambda(func(ctx context.Context, input agentContext) (agentOutput, error) {
		return runAgent(ctx, bearAgent, input)
	}))
	bearNode.AddInput(compose.START, compose.MapFields("Symbol", "Symbol"), compose.MapFields("Name", "Name"))
	bearNode.AddInput("holdings", compose.MapFields("Positions", "HoldingsPositions"), compose.MapFields("TotalValue", "HoldingsTotalValue"))
	bearNode.AddInput("holdings_market", compose.MapFields("Quotes", "HoldingsMarket"))
	bearNode.AddInput("target_market", compose.MapFields("Quote", "TargetMarket"))
	bearNode.AddInput("news_summary", compose.MapFields("Summary", "NewsSummary"), compose.MapFields("Sentiment", "NewsSentiment"), compose.MapFields("Items", "NewsItems"))

	debateNode := wf.AddLambdaNode("debate", compose.InvokableLambda(func(ctx context.Context, input debateInput) (agentOutput, error) {
		return runDebateAgent(ctx, debateAgent, input)
	}))
	debateNode.AddInput("bull", compose.MapFields("Content", "Bull"))
	debateNode.AddInput("bear", compose.MapFields("Content", "Bear"))
	debateNode.AddInput(compose.START, compose.MapFields("Symbol", "Symbol"), compose.MapFields("Name", "Name"))
	debateNode.AddInput("news_summary", compose.MapFields("Summary", "NewsSummary"), compose.MapFields("Sentiment", "NewsSentiment"))

	summaryNode := wf.AddLambdaNode("summary", compose.InvokableLambda(func(ctx context.Context, input summaryInput) (agentOutput, error) {
		return runSummaryAgent(ctx, summaryAgent, input)
	}))
	summaryNode.AddInput("debate", compose.MapFields("Content", "Debate"))
	summaryNode.AddInput("bull", compose.MapFields("Content", "Bull"))
	summaryNode.AddInput("bear", compose.MapFields("Content", "Bear"))
	summaryNode.AddInput(compose.START, compose.MapFields("Symbol", "Symbol"), compose.MapFields("Name", "Name"))
	summaryNode.AddInput("holdings", compose.MapFields("Positions", "HoldingsPositions"), compose.MapFields("TotalValue", "HoldingsTotalValue"))
	summaryNode.AddInput("target_market", compose.MapFields("Quote", "TargetMarket"))
	summaryNode.AddInput("news_summary", compose.MapFields("Summary", "NewsSummary"), compose.MapFields("Sentiment", "NewsSentiment"))

	wf.End().AddInput("holdings", compose.ToField("Holdings"))
	wf.End().AddInput("holdings_market", compose.MapFields("Quotes", "HoldingsMarket"))
	wf.End().AddInput("target_market", compose.MapFields("Quote", "TargetMarket"))
	wf.End().AddInput("news_summary", compose.ToField("News"))
	wf.End().AddInput("bull", compose.MapFields("Content", "BullAnalysis"))
	wf.End().AddInput("bear", compose.MapFields("Content", "BearAnalysis"))
	wf.End().AddInput("debate", compose.MapFields("Content", "DebateAnalysis"))
	wf.End().AddInput("summary", compose.MapFields("Content", "Summary"))

	runner, err := wf.Compile(ctx)
	if err != nil {
		return nil, err
	}
	return &StockWorkflow{runner: runner}, nil
}

func (w *StockWorkflow) Run(ctx context.Context, input StockWorkflowInput) (StockWorkflowOutput, error) {
	if w == nil || w.runner == nil {
		return StockWorkflowOutput{}, errors.New("workflow_not_initialized")
	}
	return w.runner.Invoke(ctx, input)
}

type holdingsInput struct {
	AccountID int64
}

type holdingsMarketInput struct {
	Positions []HoldingPosition
}

type holdingsMarketOutput struct {
	Quotes []MarketMove
}

type marketInput struct {
	Symbol string
	Name   string
}

type targetMarketOutput struct {
	Quote MarketMove
}

type newsInput struct {
	Symbol             string
	Name               string
	StockNewsRoutes    []string
	IndustryNewsRoutes []string
	NewsLimit          int
}

type newsFetchOutput struct {
	Items []rsshub.Item
}

type newsSummaryInput struct {
	Items []rsshub.Item
}

type agentContext struct {
	Symbol             string
	Name               string
	HoldingsPositions  []HoldingPosition
	HoldingsTotalValue float64
	HoldingsMarket     []MarketMove
	TargetMarket       MarketMove
	NewsSummary        string
	NewsSentiment      string
	NewsItems          []rsshub.Item
}

type agentOutput struct {
	Content string
}

type debateInput struct {
	Symbol        string
	Name          string
	Bull          string
	Bear          string
	NewsSummary   string
	NewsSentiment string
}

type summaryInput struct {
	Symbol             string
	Name               string
	Bull               string
	Bear               string
	Debate             string
	HoldingsPositions  []HoldingPosition
	HoldingsTotalValue float64
	TargetMarket       MarketMove
	NewsSummary        string
	NewsSentiment      string
}

func buildHoldings(ctx context.Context, repo *investment.Repository, accountID int64) (HoldingsOutput, error) {
	if repo == nil {
		return HoldingsOutput{}, nil
	}
	if accountID == 0 {
		var err error
		accountID, err = repo.DefaultAccountID(ctx)
		if err != nil {
			return HoldingsOutput{}, err
		}
	}
	positions, err := repo.ListPositions(ctx, accountID)
	if err != nil {
		return HoldingsOutput{}, err
	}
	result := HoldingsOutput{Positions: make([]HoldingPosition, 0, len(positions))}
	var total float64
	for _, p := range positions {
		price := p.AvgCost
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			price = *p.MarketPrice
		}
		value := p.Quantity * price
		total += value
		result.Positions = append(result.Positions, HoldingPosition{
			Symbol:      p.Instrument.Symbol,
			Name:        p.Instrument.Name,
			Quantity:    p.Quantity,
			AvgCost:     p.AvgCost,
			MarketPrice: price,
			Value:       value,
		})
	}
	result.TotalValue = total
	if total > 0 {
		for i := range result.Positions {
			result.Positions[i].Ratio = result.Positions[i].Value / total
		}
	}
	return result, nil
}

func buildHoldingsMarket(ctx context.Context, registry *tool.Registry, holdings holdingsMarketInput) (holdingsMarketOutput, error) {
	quotes := make([]MarketMove, 0, len(holdings.Positions))
	for _, position := range holdings.Positions {
		quotes = append(quotes, fetchQuote(ctx, registry, position.Symbol, position.Name))
	}
	return holdingsMarketOutput{Quotes: quotes}, nil
}

func buildTargetMarket(ctx context.Context, registry *tool.Registry, input marketInput) (targetMarketOutput, error) {
	if strings.TrimSpace(input.Symbol) == "" {
		return targetMarketOutput{}, errors.New("missing_symbol")
	}
	return targetMarketOutput{Quote: fetchQuote(ctx, registry, input.Symbol, input.Name)}, nil
}

func fetchQuote(ctx context.Context, registry *tool.Registry, symbol string, name string) MarketMove {
	if registry == nil || strings.TrimSpace(symbol) == "" {
		return MarketMove{Symbol: symbol, Name: name, Error: "tool_registry_not_initialized"}
	}
	call := tool.ToolCall{
		Name: "eastmoney",
		Arguments: map[string]interface{}{
			"action": "get_stock_quote",
			"code":   symbol,
		},
	}
	result, err := registry.Execute(ctx, call)
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	move := MarketMove{Symbol: symbol, Name: name, Error: result.Error}
	if result.Output != nil {
		if quote, ok := result.Output.(tool.StockQuote); ok {
			move.Symbol = quote.Code
			move.Name = quote.Name
			move.Price = quote.Price
			move.Change = quote.Change
			move.ChangeRate = quote.ChangeRate
			return move
		}
		raw, _ := json.Marshal(result.Output)
		_ = json.Unmarshal(raw, &move)
		if move.Symbol == "" {
			move.Symbol = symbol
		}
		if move.Name == "" {
			move.Name = name
		}
	}
	return move
}

func fetchNews(ctx context.Context, registry *tool.Registry, input newsInput) (newsFetchOutput, error) {
	routes := append([]string{}, input.StockNewsRoutes...)
	routes = append(routes, input.IndustryNewsRoutes...)
	if len(routes) == 0 {
		return newsFetchOutput{}, nil
	}
	limit := input.NewsLimit
	if limit <= 0 {
		limit = 10
	}
	items := make([]rsshub.Item, 0)
	for _, route := range routes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}
		call := tool.ToolCall{
			Name: "rsshub",
			Arguments: map[string]interface{}{
				"action": "fetch_feed",
				"route":  route,
				"limit":  limit,
			},
		}
		result, err := registry.Execute(ctx, call)
		if err != nil {
			continue
		}
		if out, ok := result.Output.([]rsshub.Item); ok {
			items = append(items, out...)
			continue
		}
		raw, _ := json.Marshal(result.Output)
		var parsed []rsshub.Item
		if err := json.Unmarshal(raw, &parsed); err == nil {
			items = append(items, parsed...)
		}
	}
	return newsFetchOutput{Items: items}, nil
}

func summarizeNews(ctx context.Context, model model.ToolCallingChatModel, items []rsshub.Item) (NewsSummaryOutput, error) {
	if len(items) == 0 {
		return NewsSummaryOutput{Items: items}, nil
	}
	trimmed := compactNewsItems(items, 10)
	payload, _ := json.Marshal(trimmed)
	systemPrompt := "你是财经新闻摘要助手，请输出严格JSON：{\"summary\":\"...\",\"sentiment\":\"利好|利空|中性\"}"
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(string(payload)),
	}
	resp, err := model.Generate(ctx, msgs)
	if err != nil {
		return NewsSummaryOutput{Items: items}, err
	}
	if resp == nil {
		return NewsSummaryOutput{Items: trimmed}, nil
	}
	summary, sentiment, ok := parseNewsSummary(resp.Content)
	if !ok {
		summary = strings.TrimSpace(resp.Content)
	}
	if summary != "" && sentiment == "" {
		sentiment = "中性"
	}
	return NewsSummaryOutput{Items: trimmed, Summary: summary, Sentiment: sentiment}, nil
}

func parseNewsSummary(content string) (string, string, bool) {
	type parsedSummary struct {
		Summary   string `json:"summary"`
		Sentiment string `json:"sentiment"`
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", "", false
	}
	tryParse := func(raw string) (string, string, bool) {
		var parsed parsedSummary
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return "", "", false
		}
		if strings.TrimSpace(parsed.Summary) == "" && strings.TrimSpace(parsed.Sentiment) == "" {
			return "", "", false
		}
		return strings.TrimSpace(parsed.Summary), strings.TrimSpace(parsed.Sentiment), true
	}
	if summary, sentiment, ok := tryParse(content); ok {
		return summary, sentiment, true
	}
	if idx := strings.Index(content, "```"); idx >= 0 {
		end := strings.LastIndex(content, "```")
		if end > idx {
			block := strings.TrimSpace(content[idx+3 : end])
			if strings.HasPrefix(strings.ToLower(block), "json") {
				block = strings.TrimSpace(block[4:])
			}
			if summary, sentiment, ok := tryParse(block); ok {
				return summary, sentiment, true
			}
		}
	}
	if start := strings.Index(content, "{"); start >= 0 {
		end := strings.LastIndex(content, "}")
		if end > start {
			block := content[start : end+1]
			if summary, sentiment, ok := tryParse(block); ok {
				return summary, sentiment, true
			}
		}
	}
	return "", "", false
}

func compactNewsItems(items []rsshub.Item, maxItems int) []rsshub.Item {
	if maxItems <= 0 {
		maxItems = 10
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]rsshub.Item, 0, maxItems)
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		link := strings.TrimSpace(item.Link)
		if title == "" && link == "" {
			continue
		}
		key := title + "|" + link
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if len([]rune(title)) > 200 {
			runes := []rune(title)
			title = string(runes[:200])
		}
		result = append(result, rsshub.Item{
			Title:       title,
			Link:        link,
			GUID:        strings.TrimSpace(item.GUID),
			PublishedAt: item.PublishedAt,
		})
		if len(result) >= maxItems {
			break
		}
	}
	return result
}

func runAgent(ctx context.Context, ag agent.Agent, input agentContext) (agentOutput, error) {
	if ag == nil {
		return agentOutput{}, errors.New("agent_not_initialized")
	}
	payloadMap := map[string]interface{}{
		"Symbol":             input.Symbol,
		"Name":               input.Name,
		"HoldingsPositions":  input.HoldingsPositions,
		"HoldingsTotalValue": input.HoldingsTotalValue,
		"HoldingsMarket":     input.HoldingsMarket,
		"TargetMarket":       input.TargetMarket,
	}
	includeNews := strings.TrimSpace(input.NewsSummary) != "" || strings.TrimSpace(input.NewsSentiment) != "" || len(input.NewsItems) > 0
	if includeNews {
		payloadMap["NewsSummary"] = input.NewsSummary
		payloadMap["NewsSentiment"] = input.NewsSentiment
		payloadMap["NewsItems"] = input.NewsItems
	}
	payload, _ := json.Marshal(payloadMap)
	resp, err := ag.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: string(payload)}},
	})
	if err != nil {
		return agentOutput{}, err
	}
	return agentOutput{Content: resp.Message.Content}, nil
}

func runDebateAgent(ctx context.Context, ag agent.Agent, input debateInput) (agentOutput, error) {
	if ag == nil {
		return agentOutput{}, errors.New("agent_not_initialized")
	}
	payloadMap := map[string]interface{}{
		"Symbol": input.Symbol,
		"Name":   input.Name,
		"Bull":   input.Bull,
		"Bear":   input.Bear,
	}
	includeNews := strings.TrimSpace(input.NewsSummary) != "" || strings.TrimSpace(input.NewsSentiment) != ""
	if includeNews {
		payloadMap["NewsSummary"] = input.NewsSummary
		payloadMap["NewsSentiment"] = input.NewsSentiment
	}
	payload, _ := json.Marshal(payloadMap)
	resp, err := ag.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: string(payload)}},
	})
	if err != nil {
		return agentOutput{}, err
	}
	return agentOutput{Content: resp.Message.Content}, nil
}

func runSummaryAgent(ctx context.Context, ag agent.Agent, input summaryInput) (agentOutput, error) {
	if ag == nil {
		return agentOutput{}, errors.New("agent_not_initialized")
	}
	payloadMap := map[string]interface{}{
		"Symbol":             input.Symbol,
		"Name":               input.Name,
		"Bull":               input.Bull,
		"Bear":               input.Bear,
		"Debate":             input.Debate,
		"HoldingsPositions":  input.HoldingsPositions,
		"HoldingsTotalValue": input.HoldingsTotalValue,
		"TargetMarket":       input.TargetMarket,
	}
	includeNews := strings.TrimSpace(input.NewsSummary) != "" || strings.TrimSpace(input.NewsSentiment) != ""
	if includeNews {
		payloadMap["NewsSummary"] = input.NewsSummary
		payloadMap["NewsSentiment"] = input.NewsSentiment
	}
	payload, _ := json.Marshal(payloadMap)
	resp, err := ag.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: string(payload)}},
	})
	if err != nil {
		return agentOutput{}, err
	}
	return agentOutput{Content: resp.Message.Content}, nil
}
