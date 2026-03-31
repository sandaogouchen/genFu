package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/components/model"
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
	registry             *tool.Registry
	model                model.ToolCallingChatModel
	investRepo           *investment.Repository
	defaultNewsRoutes    []string
	bullAgent            agent.Agent
	bearAgent            agent.Agent
	debateAgent          agent.Agent
	summaryAgent         agent.Agent
	workflowPlannerAgent *WorkflowPlannerAgent
}

func NewStockWorkflow(ctx context.Context, model model.ToolCallingChatModel, registry *tool.Registry, investRepo *investment.Repository, defaultNewsRoutes []string) (*StockWorkflow, error) {
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
	return newStockWorkflowWithAgents(ctx, model, registry, investRepo, defaultNewsRoutes, bullAgent, bearAgent, debateAgent, summaryAgent)
}

func newStockWorkflowWithAgents(ctx context.Context, model model.ToolCallingChatModel, registry *tool.Registry, investRepo *investment.Repository, defaultNewsRoutes []string, bullAgent agent.Agent, bearAgent agent.Agent, debateAgent agent.Agent, summaryAgent agent.Agent) (*StockWorkflow, error) {
	_ = ctx
	if model == nil {
		return nil, errors.New("model_not_initialized")
	}
	defaultRoutes := normalizeRoutes(defaultNewsRoutes)
	return &StockWorkflow{
		registry:             registry,
		model:                model,
		investRepo:           investRepo,
		defaultNewsRoutes:    defaultRoutes,
		bullAgent:            bullAgent,
		bearAgent:            bearAgent,
		debateAgent:          debateAgent,
		summaryAgent:         summaryAgent,
		workflowPlannerAgent: NewWorkflowPlannerAgent(defaultRoutes, investRepo != nil),
	}, nil
}

func (w *StockWorkflow) Run(ctx context.Context, input StockWorkflowInput) (StockWorkflowOutput, error) {
	return w.RunStream(ctx, input, nil)
}

func (w *StockWorkflow) RunStream(ctx context.Context, input StockWorkflowInput, emit func(event WorkflowStreamEvent)) (StockWorkflowOutput, error) {
	if w == nil {
		return StockWorkflowOutput{}, errors.New("workflow_not_initialized")
	}
	normalizedInput, err := w.resolveWorkflowInstrument(ctx, input)
	if err != nil {
		return StockWorkflowOutput{}, err
	}
	planner := w.workflowPlannerAgent
	if planner == nil {
		planner = NewWorkflowPlannerAgent(w.defaultNewsRoutes, w.investRepo != nil)
	}
	plan := planner.Plan(normalizedInput)
	streamer := NewNodeStreamer(emit)
	streamer.EmitPlan(plan)
	for _, skipped := range plan.Skipped {
		streamer.Skip(skipped.Node, skipped.Reason)
	}

	var output StockWorkflowOutput
	var holdings HoldingsOutput
	var holdingsMarket holdingsMarketOutput
	var targetMarket targetMarketOutput
	var fetchedNews newsFetchOutput
	var newsSummary NewsSummaryOutput
	var bullAnalysis agentOutput
	var bearAnalysis agentOutput
	var debateAnalysis agentOutput
	var summaryAnalysis agentOutput

	if plan.ShouldRun(workflowNodeHoldings) {
		streamer.Start(workflowNodeHoldings)
		holdings, err = buildHoldings(ctx, w.investRepo, normalizedInput.AccountID)
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		output.Holdings = holdings
		streamer.Complete(workflowNodeHoldings, holdings)
	}

	if plan.ShouldRun(workflowNodeHoldingsMkt) {
		streamer.Start(workflowNodeHoldingsMkt)
		holdingsMarket, err = buildHoldingsMarket(ctx, w.registry, holdingsMarketInput{Positions: holdings.Positions})
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		output.HoldingsMarket = holdingsMarket.Quotes
		streamer.Complete(workflowNodeHoldingsMkt, holdingsMarket.Quotes)
	}

	if plan.ShouldRun(workflowNodeTargetMarket) {
		streamer.Start(workflowNodeTargetMarket)
		targetMarket, err = buildTargetMarket(ctx, w.registry, marketInput{
			Symbol:    normalizedInput.Symbol,
			Name:      normalizedInput.Name,
			AssetType: normalizedInput.AssetType,
		})
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		output.TargetMarket = targetMarket.Quote
		streamer.Complete(workflowNodeTargetMarket, targetMarket.Quote)
	}

	if plan.ShouldRun(workflowNodeNewsFetch) {
		streamer.Start(workflowNodeNewsFetch)
		fetchedNews, err = fetchNews(ctx, w.registry, w.defaultNewsRoutes, newsInput{
			Symbol:             normalizedInput.Symbol,
			Name:               normalizedInput.Name,
			StockNewsRoutes:    normalizedInput.StockNewsRoutes,
			IndustryNewsRoutes: normalizedInput.IndustryNewsRoutes,
			NewsLimit:          normalizedInput.NewsLimit,
		})
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		streamer.Complete(workflowNodeNewsFetch, map[string]int{"items": len(fetchedNews.Items)})
	}

	if plan.ShouldRun(workflowNodeNewsSummary) {
		streamer.Start(workflowNodeNewsSummary)
		newsSummary, err = summarizeNews(ctx, w.model, fetchedNews.Items)
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		output.News = newsSummary
		streamer.Complete(workflowNodeNewsSummary, newsSummary)
	}

	agentCtx := agentContext{
		Symbol:        normalizedInput.Symbol,
		Name:          normalizedInput.Name,
		TargetMarket:  targetMarket.Quote,
		NewsSummary:   newsSummary.Summary,
		NewsSentiment: newsSummary.Sentiment,
		NewsItems:     newsSummary.Items,
	}

	if plan.ShouldRun(workflowNodeBull) {
		streamer.Start(workflowNodeBull)
		bullAnalysis, err = runAgent(ctx, w.bullAgent, agentCtx, streamer, workflowNodeBull)
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		output.BullAnalysis = bullAnalysis.Content
		streamer.Complete(workflowNodeBull, map[string]string{"content": bullAnalysis.Content})
	}

	if plan.ShouldRun(workflowNodeBear) {
		streamer.Start(workflowNodeBear)
		bearAnalysis, err = runAgent(ctx, w.bearAgent, agentCtx, streamer, workflowNodeBear)
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		output.BearAnalysis = bearAnalysis.Content
		streamer.Complete(workflowNodeBear, map[string]string{"content": bearAnalysis.Content})
	}

	if plan.ShouldRun(workflowNodeDebate) {
		streamer.Start(workflowNodeDebate)
		debateAnalysis, err = runDebateAgent(ctx, w.debateAgent, debateInput{
			Symbol:        normalizedInput.Symbol,
			Name:          normalizedInput.Name,
			Bull:          bullAnalysis.Content,
			Bear:          bearAnalysis.Content,
			NewsSummary:   newsSummary.Summary,
			NewsSentiment: newsSummary.Sentiment,
		}, streamer, workflowNodeDebate)
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		output.DebateAnalysis = debateAnalysis.Content
		streamer.Complete(workflowNodeDebate, map[string]string{"content": debateAnalysis.Content})
	}

	if plan.ShouldRun(workflowNodeSummary) {
		streamer.Start(workflowNodeSummary)
		summaryAnalysis, err = runSummaryAgent(ctx, w.summaryAgent, summaryInput{
			Symbol:             normalizedInput.Symbol,
			Name:               normalizedInput.Name,
			Bull:               bullAnalysis.Content,
			Bear:               bearAnalysis.Content,
			Debate:             debateAnalysis.Content,
			HoldingsPositions:  holdings.Positions,
			HoldingsTotalValue: holdings.TotalValue,
			TargetMarket:       targetMarket.Quote,
			NewsSummary:        newsSummary.Summary,
			NewsSentiment:      newsSummary.Sentiment,
		}, streamer, workflowNodeSummary)
		if err != nil {
			return StockWorkflowOutput{}, err
		}
		output.Summary = summaryAnalysis.Content
		streamer.Complete(workflowNodeSummary, map[string]string{"content": summaryAnalysis.Content})
	}

	return output, nil
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
	Symbol    string
	Name      string
	AssetType string
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
	Symbol        string
	Name          string
	TargetMarket  MarketMove
	NewsSummary   string
	NewsSentiment string
	NewsItems     []rsshub.Item
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
			AssetType:   p.Instrument.AssetType,
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
		quotes = append(quotes, fetchQuoteByAsset(ctx, registry, position.Symbol, position.Name, position.AssetType))
	}
	return holdingsMarketOutput{Quotes: quotes}, nil
}

func buildTargetMarket(ctx context.Context, registry *tool.Registry, input marketInput) (targetMarketOutput, error) {
	if strings.TrimSpace(input.Symbol) == "" {
		return targetMarketOutput{}, errors.New("missing_symbol")
	}
	return targetMarketOutput{Quote: fetchQuoteByAsset(ctx, registry, input.Symbol, input.Name, input.AssetType)}, nil
}

func fetchStockQuote(ctx context.Context, registry *tool.Registry, symbol string, name string) MarketMove {
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

func fetchFundQuote(ctx context.Context, registry *tool.Registry, symbol string, name string) MarketMove {
	move := MarketMove{Symbol: symbol, Name: name}
	if registry == nil || strings.TrimSpace(symbol) == "" {
		move.Error = "tool_registry_not_initialized"
		return move
	}
	call := tool.ToolCall{
		Name: "marketdata",
		Arguments: map[string]interface{}{
			"action": "get_fund_intraday",
			"code":   symbol,
		},
	}
	result, err := registry.Execute(ctx, call)
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	if result.Error != "" {
		move.Error = result.Error
		return move
	}
	price, err := parseLastPositiveIntradayPrice(result.Output)
	if err != nil {
		move.Error = err.Error()
		return move
	}
	move.Price = price
	return move
}

func fetchQuoteByAsset(ctx context.Context, registry *tool.Registry, symbol string, name string, assetType string) MarketMove {
	switch normalizeAssetType(assetType) {
	case "fund":
		return fetchFundQuote(ctx, registry, symbol, name)
	case "stock":
		return fetchStockQuote(ctx, registry, symbol, name)
	default:
		stockMove := fetchStockQuote(ctx, registry, symbol, name)
		if stockMove.Error == "" && stockMove.Price > 0 {
			return stockMove
		}
		fundMove := fetchFundQuote(ctx, registry, symbol, name)
		if fundMove.Error == "" && fundMove.Price > 0 {
			return fundMove
		}
		if stockMove.Error == "" {
			return fundMove
		}
		if fundMove.Error == "" {
			return stockMove
		}
		stockMove.Error = stockMove.Error + " | fund_fallback:" + fundMove.Error
		return stockMove
	}
}

func fetchNews(ctx context.Context, registry *tool.Registry, defaultRoutes []string, input newsInput) (newsFetchOutput, error) {
	if registry == nil {
		return newsFetchOutput{}, errors.New("tool_registry_not_initialized")
	}
	routes := append([]string{}, input.StockNewsRoutes...)
	routes = append(routes, input.IndustryNewsRoutes...)
	if len(routes) == 0 {
		routes = append(routes, defaultRoutes...)
	}
	routes = normalizeRoutes(routes)
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

func parseLastPositiveIntradayPrice(output interface{}) (float64, error) {
	if output == nil {
		return 0, errors.New("empty_intraday_price")
	}
	switch points := output.(type) {
	case []tool.IntradayPoint:
		return pickLastPositiveIntraday(points)
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return 0, err
	}
	var decoded []tool.IntradayPoint
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return 0, err
	}
	return pickLastPositiveIntraday(decoded)
}

func pickLastPositiveIntraday(points []tool.IntradayPoint) (float64, error) {
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Price > 0 {
			return points[i].Price, nil
		}
	}
	return 0, errors.New("empty_intraday_price")
}

func normalizeAssetType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(normalized, "fund"), strings.Contains(normalized, "基金"):
		return "fund"
	case strings.Contains(normalized, "stock"), strings.Contains(normalized, "股票"):
		return "stock"
	default:
		return ""
	}
}

func normalizeRoutes(routes []string) []string {
	normalized := make([]string, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		trimmed := strings.TrimSpace(route)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

type workflowSearchInstrument struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	AssetType string `json:"asset_type"`
}

func (w *StockWorkflow) resolveWorkflowInstrument(ctx context.Context, input StockWorkflowInput) (StockWorkflowInput, error) {
	input.Symbol = strings.TrimSpace(input.Symbol)
	input.Name = strings.TrimSpace(input.Name)
	if input.Symbol == "" {
		return StockWorkflowInput{}, errors.New("missing_symbol")
	}
	if looksLikeQuoteCode(input.Symbol) {
		if input.Name == "" {
			input.Name = input.Symbol
		}
		return input, nil
	}
	if w == nil || w.registry == nil {
		return StockWorkflowInput{}, errors.New("tool_registry_not_initialized")
	}

	targetName := input.Name
	if targetName == "" {
		targetName = input.Symbol
	}
	call := tool.ToolCall{
		Name: "investment",
		Arguments: map[string]interface{}{
			"action": "search_instruments",
			"query":  input.Symbol,
			"limit":  30,
		},
	}
	result, err := w.registry.Execute(ctx, call)
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	if result.Error != "" {
		return StockWorkflowInput{}, fmt.Errorf("fund_search_failed:%s", result.Error)
	}
	candidates, err := parseWorkflowSearchInstruments(result.Output)
	if err != nil {
		return StockWorkflowInput{}, fmt.Errorf("fund_search_parse_failed:%w", err)
	}
	normalizedTarget := normalizeSearchName(targetName)
	for _, item := range candidates {
		if normalizeSearchName(item.Name) != normalizedTarget {
			continue
		}
		input.Symbol = item.Symbol
		input.Name = item.Name
		input.AssetType = "fund"
		return input, nil
	}
	return StockWorkflowInput{}, fmt.Errorf("fund_search_not_exact_match:%s", targetName)
}

func parseWorkflowSearchInstruments(output interface{}) ([]workflowSearchInstrument, error) {
	if output == nil {
		return nil, nil
	}
	switch rows := output.(type) {
	case []map[string]interface{}:
		return collectWorkflowSearchItems(rows), nil
	case []interface{}:
		records := make([]map[string]interface{}, 0, len(rows))
		for _, row := range rows {
			if rec, ok := row.(map[string]interface{}); ok {
				records = append(records, rec)
			}
		}
		return collectWorkflowSearchItems(records), nil
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	var decoded []workflowSearchInstrument
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	filtered := make([]workflowSearchInstrument, 0, len(decoded))
	for _, item := range decoded {
		symbol := strings.TrimSpace(item.Symbol)
		name := strings.TrimSpace(item.Name)
		if symbol == "" || name == "" {
			continue
		}
		filtered = append(filtered, workflowSearchInstrument{Symbol: symbol, Name: name, AssetType: strings.TrimSpace(item.AssetType)})
	}
	return filtered, nil
}

func collectWorkflowSearchItems(records []map[string]interface{}) []workflowSearchInstrument {
	items := make([]workflowSearchInstrument, 0, len(records))
	for _, rec := range records {
		symbol := strings.TrimSpace(fmt.Sprintf("%v", rec["symbol"]))
		if strings.EqualFold(symbol, "<nil>") {
			symbol = ""
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", rec["name"]))
		if strings.EqualFold(name, "<nil>") {
			name = ""
		}
		assetType := strings.TrimSpace(fmt.Sprintf("%v", rec["asset_type"]))
		if strings.EqualFold(assetType, "<nil>") {
			assetType = ""
		}
		if symbol == "" || name == "" {
			continue
		}
		items = append(items, workflowSearchInstrument{Symbol: symbol, Name: name, AssetType: assetType})
	}
	return items
}

func normalizeSearchName(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, trimmed)
}

func looksLikeQuoteCode(value string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	if len(normalized) == 6 && isAllDigits(normalized) {
		return true
	}
	if len(normalized) == 8 {
		prefix := normalized[:2]
		if (prefix == "SH" || prefix == "SZ" || prefix == "BJ") && isAllDigits(normalized[2:]) {
			return true
		}
	}
	return false
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
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

func runAgent(ctx context.Context, ag agent.Agent, input agentContext, streamer *NodeStreamer, nodeName string) (agentOutput, error) {
	if ag == nil {
		return agentOutput{}, errors.New("agent_not_initialized")
	}
	payloadMap := map[string]interface{}{
		"Symbol":       input.Symbol,
		"Name":         input.Name,
		"TargetMarket": input.TargetMarket,
	}
	includeNews := strings.TrimSpace(input.NewsSummary) != "" || strings.TrimSpace(input.NewsSentiment) != "" || len(input.NewsItems) > 0
	if includeNews {
		payloadMap["NewsSummary"] = input.NewsSummary
		payloadMap["NewsSentiment"] = input.NewsSentiment
		payloadMap["NewsItems"] = input.NewsItems
	}
	return runAgentWithPayload(ctx, ag, payloadMap, streamer, nodeName)
}

func runDebateAgent(ctx context.Context, ag agent.Agent, input debateInput, streamer *NodeStreamer, nodeName string) (agentOutput, error) {
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
	return runAgentWithPayload(ctx, ag, payloadMap, streamer, nodeName)
}

func runSummaryAgent(ctx context.Context, ag agent.Agent, input summaryInput, streamer *NodeStreamer, nodeName string) (agentOutput, error) {
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
	return runAgentWithPayload(ctx, ag, payloadMap, streamer, nodeName)
}

type streamCapableAgent interface {
	HandleStream(ctx context.Context, req generate.GenerateRequest) (<-chan generate.GenerateEvent, error)
}

func runAgentWithPayload(ctx context.Context, ag agent.Agent, payloadMap map[string]interface{}, streamer *NodeStreamer, nodeName string) (agentOutput, error) {
	payload, _ := json.Marshal(payloadMap)
	req := generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: string(payload)}},
	}
	if streamingAgent, ok := ag.(streamCapableAgent); ok {
		events, err := streamingAgent.HandleStream(ctx, req)
		if err != nil {
			return agentOutput{}, err
		}
		var finalContent strings.Builder
		var hasFinal bool
		for event := range events {
			switch event.Type {
			case "delta":
				streamer.Delta(nodeName, event.Delta)
				finalContent.WriteString(event.Delta)
			case "message":
				if event.Message != nil {
					finalContent.Reset()
					finalContent.WriteString(event.Message.Content)
					hasFinal = true
				}
			case "error":
				if strings.TrimSpace(event.Delta) != "" {
					return agentOutput{}, errors.New(event.Delta)
				}
				return agentOutput{}, errors.New("stream_agent_error")
			}
		}
		content := strings.TrimSpace(finalContent.String())
		if !hasFinal {
			content = finalContent.String()
		}
		return agentOutput{Content: content}, nil
	}
	resp, err := ag.Handle(ctx, req)
	if err != nil {
		return agentOutput{}, err
	}
	return agentOutput{Content: resp.Message.Content}, nil
}
