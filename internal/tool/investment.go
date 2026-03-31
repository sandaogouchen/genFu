package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"genFu/internal/investment"
)

type InvestmentTool struct {
	svc              *investment.Service
	instrumentSearch instrumentSearchProvider
	marketData       MarketDataTool
	priceResolver    PortfolioPriceResolver
}

type PortfolioPriceResolver interface {
	ResolveStockPrice(ctx context.Context, symbol string) (float64, error)
	ResolveFundPrice(ctx context.Context, symbol string) (float64, error)
}

type instrumentSearchProvider interface {
	SearchInstruments(ctx context.Context, query string, limit int) ([]SearchItem, error)
	SearchStockByCode(ctx context.Context, code string) (*SearchItem, error)
}

type marketDataExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error)
}

type defaultPortfolioPriceResolver struct {
	marketData marketDataExecutor
}

const (
	portfolioPriceSourceRealtime = "realtime"
	portfolioPriceSourceStored   = "stored"
	portfolioPriceSourceAvgCost  = "avg_cost"
	maxQuoteConcurrency          = 4
	perQuoteTimeout              = 4 * time.Second
)

type PortfolioSnapshotPriceFailure struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	AssetType string `json:"asset_type"`
	Reason    string `json:"reason"`
}

type PortfolioSnapshotPosition struct {
	ID               int64                 `json:"id"`
	AccountID        int64                 `json:"account_id"`
	Instrument       investment.Instrument `json:"instrument"`
	Quantity         float64               `json:"quantity"`
	AvgCost          float64               `json:"avg_cost"`
	MarketPrice      *float64              `json:"market_price,omitempty"`
	OperationGuideID *int64                `json:"operation_guide_id,omitempty"`
	CurrentPrice     float64               `json:"current_price"`
	PriceSource      string                `json:"price_source"`
	Cost             float64               `json:"cost"`
	MarketValue      float64               `json:"market_value"`
	PnL              float64               `json:"pnl"`
	PnLPct           float64               `json:"pnl_pct"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

type PortfolioSnapshotSummary struct {
	AccountID     int64   `json:"account_id"`
	PositionCount int64   `json:"position_count"`
	TradeCount    int64   `json:"trade_count"`
	TotalCost     float64 `json:"total_cost"`
	TotalValue    float64 `json:"total_value"`
	TotalPnL      float64 `json:"total_pnl"`
	PnLPct        float64 `json:"pnl_pct"`
}

type PortfolioSnapshot struct {
	Positions      []PortfolioSnapshotPosition     `json:"positions"`
	Summary        PortfolioSnapshotSummary        `json:"summary"`
	RefreshedAt    string                          `json:"refreshed_at"`
	HasStalePrices bool                            `json:"has_stale_prices"`
	PriceFailures  []PortfolioSnapshotPriceFailure `json:"price_failures,omitempty"`
}

func NewInvestmentTool(svc *investment.Service) InvestmentTool {
	return NewInvestmentToolWithEastMoney(svc, NewEastMoneyTool())
}

func NewInvestmentToolWithEastMoney(svc *investment.Service, searchProvider instrumentSearchProvider) InvestmentTool {
	marketData := NewMarketDataTool(svc)
	return NewInvestmentToolWithResolver(svc, searchProvider, marketData, newDefaultPortfolioPriceResolver(marketData))
}

func NewInvestmentToolWithResolver(svc *investment.Service, searchProvider instrumentSearchProvider, marketData MarketDataTool, resolver PortfolioPriceResolver) InvestmentTool {
	if searchProvider == nil {
		searchProvider = NewEastMoneyTool()
	}
	if resolver == nil {
		resolver = newDefaultPortfolioPriceResolver(marketData)
	}
	return InvestmentTool{
		svc:              svc,
		instrumentSearch: searchProvider,
		marketData:       marketData,
		priceResolver:    resolver,
	}
}

func newDefaultPortfolioPriceResolver(marketData marketDataExecutor) PortfolioPriceResolver {
	return defaultPortfolioPriceResolver{
		marketData: marketData,
	}
}

func (t InvestmentTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "investment",
		Description: "manage investment portfolio data",
		Params: map[string]string{
			"action":        "string",
			"name":          "string",
			"base_currency": "string",
			"symbol":        "string",
			"asset_type":    "string",
			"instrument_id": "number",
			"quantity":      "number",
			"avg_cost":      "number",
			"market_price":  "number",
			"side":          "string",
			"price":         "number",
			"fee":           "number",
			"trade_at":      "string",
			"note":          "string",
			"amount":        "number",
			"currency":      "string",
			"flow_type":     "string",
			"flow_at":       "string",
			"total_value":   "number",
			"total_cost":    "number",
			"valuation_at":  "string",
			"limit":         "number",
			"offset":        "number",
			"query":         "string",
			"value":         "number",
			"cost":          "number",
			"pnl":           "number",
			"current_value": "number",
		},
		Required: []string{"action"},
	}
}

func (t InvestmentTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	if t.svc == nil {
		return ToolResult{Name: "investment", Error: "service_not_initialized"}, errors.New("service_not_initialized")
	}
	action, err := requireString(args, "action")
	if err != nil {
		return ToolResult{Name: "investment", Error: err.Error()}, err
	}
	switch action {
	case "help":
		return ToolResult{
			Name: "investment",
			Output: map[string]interface{}{
				"actions": []string{
					"create_user",
					"create_account",
					"upsert_instrument",
					"set_position",
					"record_trade",
					"record_cash_flow",
					"record_valuation",
					"analyze_pnl",
					"get_portfolio_summary",
					"get_portfolio_snapshot",
					"list_positions",
					"list_fund_holdings",
					"get_position",
					"delete_position",
					"list_trades",
					"search_instruments",
					"add_position_by_value",
					"add_position_by_cost",
					"add_position_by_pnl",
					"add_position_simple",
					"add_position_by_quantity",
				},
			},
		}, nil
	case "create_user":
		name, err := requireString(args, "name")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		user, err := t.svc.CreateUser(ctx, name)
		return ToolResult{Name: "investment", Output: user, Error: errorString(err)}, err
	case "create_account":
		userID, err := resolveUserID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		name, err := requireString(args, "name")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		baseCurrency := optionalString(args, "base_currency")
		account, err := t.svc.CreateAccount(ctx, userID, name, baseCurrency)
		return ToolResult{Name: "investment", Output: account, Error: errorString(err)}, err
	case "upsert_instrument":
		symbol, err := requireString(args, "symbol")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		name := optionalString(args, "name")
		assetType := optionalString(args, "asset_type")
		instrument, err := t.svc.UpsertInstrument(ctx, symbol, name, assetType)
		return ToolResult{Name: "investment", Output: instrument, Error: errorString(err)}, err
	case "set_position":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		instrumentID, err := requireInt64(args, "instrument_id")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		quantity, err := requireFloat64(args, "quantity")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		avgCost, err := requireFloat64(args, "avg_cost")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		marketPrice, err := optionalFloat64(args, "market_price")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		position, err := t.svc.SetPosition(ctx, accountID, instrumentID, quantity, avgCost, marketPrice)
		return ToolResult{Name: "investment", Output: position, Error: errorString(err)}, err
	case "record_trade":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		instrumentID, err := requireInt64(args, "instrument_id")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		side, err := requireString(args, "side")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		quantity, err := requireFloat64(args, "quantity")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		price, err := requireFloat64(args, "price")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		fee, err := optionalFloat64Value(args, "fee")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		tradeAt, err := optionalTime(args, "trade_at")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		note := optionalString(args, "note")
		trade, position, err := t.svc.RecordTrade(ctx, accountID, instrumentID, side, quantity, price, fee, tradeAt, note)
		return ToolResult{
			Name: "investment",
			Output: map[string]interface{}{
				"trade":    trade,
				"position": position,
			},
			Error: errorString(err),
		}, err
	case "record_cash_flow":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		amount, err := requireFloat64(args, "amount")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		flowType, err := requireString(args, "flow_type")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		currency := optionalString(args, "currency")
		flowAt, err := optionalTime(args, "flow_at")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		note := optionalString(args, "note")
		flow, err := t.svc.RecordCashFlow(ctx, accountID, amount, currency, flowType, flowAt, note)
		return ToolResult{Name: "investment", Output: flow, Error: errorString(err)}, err
	case "record_valuation":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		totalValue, err := requireFloat64(args, "total_value")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		totalCost, err := requireFloat64(args, "total_cost")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		valuationAt, err := optionalTime(args, "valuation_at")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		val, err := t.svc.RecordValuation(ctx, accountID, totalValue, totalCost, valuationAt)
		return ToolResult{Name: "investment", Output: val, Error: errorString(err)}, err
	case "get_portfolio_summary":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		summary, err := t.svc.GetPortfolioSummary(ctx, accountID)
		return ToolResult{Name: "investment", Output: summary, Error: errorString(err)}, err
	case "get_portfolio_snapshot":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		snapshot, err := t.GetPortfolioSnapshot(ctx, accountID)
		return ToolResult{Name: "investment", Output: snapshot, Error: errorString(err)}, err
	case "analyze_pnl":
		accountID, err := requireInt64(args, "account_id")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		report, err := t.svc.AnalyzePnL(ctx, accountID)
		return ToolResult{Name: "investment", Output: report, Error: errorString(err)}, err
	case "list_positions":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		positions, err := t.svc.ListPositions(ctx, accountID)
		return ToolResult{Name: "investment", Output: positions, Error: errorString(err)}, err
	case "list_fund_holdings":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		assetType := strings.TrimSpace(strings.ToLower(optionalString(args, "asset_type")))
		positions, err := t.svc.ListPositions(ctx, accountID)
		if err != nil {
			return ToolResult{Name: "investment", Error: errorString(err)}, err
		}
		filtered := make([]investment.Position, 0, len(positions))
		for _, pos := range positions {
			if matchAssetType(pos.Instrument.AssetType, assetType) {
				filtered = append(filtered, pos)
			}
		}
		return ToolResult{Name: "investment", Output: filtered, Error: ""}, nil
	case "get_position":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		instrumentID, err := requireInt64(args, "instrument_id")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		position, err := t.svc.GetPosition(ctx, accountID, instrumentID)
		return ToolResult{Name: "investment", Output: position, Error: errorString(err)}, err
	case "delete_position":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		instrumentID, err := requireInt64(args, "instrument_id")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		err = t.svc.DeletePosition(ctx, accountID, instrumentID)
		return ToolResult{Name: "investment", Output: map[string]bool{"deleted": err == nil}, Error: errorString(err)}, err
	case "list_trades":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		limit, _ := optionalInt(args, "limit")
		offset, _ := optionalInt(args, "offset")
		trades, err := t.svc.ListTrades(ctx, accountID, limit, offset)
		return ToolResult{Name: "investment", Output: trades, Error: errorString(err)}, err
	case "search_instruments":
		query, err := requireString(args, "query")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		limit, _ := optionalInt(args, "limit")
		if t.instrumentSearch == nil {
			return ToolResult{Name: "investment", Error: "instrument_search_provider_not_initialized"}, errors.New("instrument_search_provider_not_initialized")
		}
		fundResults, fundErr := t.instrumentSearch.SearchInstruments(ctx, query, limit)
		stockResult := searchStockInstrumentByCode(ctx, t.instrumentSearch, query)
		if fundErr != nil && stockResult == nil {
			return ToolResult{Name: "investment", Error: fundErr.Error()}, fundErr
		}
		instruments := buildSearchInstrumentRecords(stockResult, fundResults, limit)
		return ToolResult{Name: "investment", Output: instruments, Error: ""}, nil
	case "add_position_by_value":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		symbol, err := requireString(args, "symbol")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		name := optionalString(args, "name")
		assetType := optionalString(args, "asset_type")
		value, err := requireFloat64(args, "value")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		avgCost, err := optionalFloat64(args, "avg_cost")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		marketPrice, err := requireFloat64(args, "market_price")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		position, err := t.svc.AddPositionByValue(ctx, accountID, symbol, name, assetType, value, avgCost, marketPrice)
		return ToolResult{Name: "investment", Output: position, Error: errorString(err)}, err
	case "add_position_by_cost":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		symbol, err := requireString(args, "symbol")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		name := optionalString(args, "name")
		assetType := optionalString(args, "asset_type")
		cost, err := requireFloat64(args, "cost")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		avgCost, err := requireFloat64(args, "avg_cost")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		marketPrice, err := optionalFloat64(args, "market_price")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		position, err := t.svc.AddPositionByCost(ctx, accountID, symbol, name, assetType, cost, avgCost, marketPrice)
		return ToolResult{Name: "investment", Output: position, Error: errorString(err)}, err
	case "add_position_by_pnl":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		symbol, err := requireString(args, "symbol")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		name := optionalString(args, "name")
		assetType := optionalString(args, "asset_type")
		pnl, err := requireFloat64(args, "pnl")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		avgCost, err := requireFloat64(args, "avg_cost")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		marketPrice, err := requireFloat64(args, "market_price")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		position, err := t.svc.AddPositionByPnL(ctx, accountID, symbol, name, assetType, pnl, avgCost, marketPrice)
		return ToolResult{Name: "investment", Output: position, Error: errorString(err)}, err
	case "add_position_simple":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		symbol, err := requireString(args, "symbol")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		name := optionalString(args, "name")
		assetType := optionalString(args, "asset_type")
		cost, err := requireFloat64(args, "cost")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		currentValue, err := requireFloat64(args, "current_value")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		marketPrice, _ := optionalFloat64Value(args, "market_price")
		position, err := t.svc.AddPositionSimple(ctx, accountID, symbol, name, assetType, cost, currentValue, marketPrice)
		return ToolResult{Name: "investment", Output: position, Error: errorString(err)}, err
	case "add_position_by_quantity":
		accountID, err := resolveAccountID(ctx, t.svc, args)
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		symbol, err := requireString(args, "symbol")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		name := optionalString(args, "name")
		assetType := optionalString(args, "asset_type")
		quantity, err := requireFloat64(args, "quantity")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		avgCost, err := requireFloat64(args, "avg_cost")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		marketPrice, err := optionalFloat64(args, "market_price")
		if err != nil {
			return ToolResult{Name: "investment", Error: err.Error()}, err
		}
		symbol, name = normalizeInstrumentIdentity(symbol, name)
		position, err := t.svc.AddPositionByQuantity(ctx, accountID, symbol, name, assetType, quantity, avgCost, marketPrice)
		return ToolResult{Name: "investment", Output: position, Error: errorString(err)}, err
	default:
		return ToolResult{Name: "investment", Error: "unsupported_action"}, errors.New("unsupported_action")
	}
}

func (t InvestmentTool) GetPortfolioSnapshot(ctx context.Context, accountID int64) (PortfolioSnapshot, error) {
	positions, err := t.svc.ListPositions(ctx, accountID)
	if err != nil {
		return PortfolioSnapshot{}, err
	}

	portfolioSummary, err := t.svc.GetPortfolioSummary(ctx, accountID)
	if err != nil {
		return PortfolioSnapshot{}, err
	}

	snapshot := PortfolioSnapshot{
		Positions: make([]PortfolioSnapshotPosition, len(positions)),
		Summary: PortfolioSnapshotSummary{
			AccountID:     accountID,
			PositionCount: portfolioSummary.PositionCount,
			TradeCount:    portfolioSummary.TradeCount,
		},
		RefreshedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if len(positions) == 0 {
		return snapshot, nil
	}

	resolver := t.priceResolver
	if resolver == nil {
		resolver = newDefaultPortfolioPriceResolver(t.marketData)
	}

	type quoteResult struct {
		price   float64
		source  string
		failure string
	}
	results := make([]quoteResult, len(positions))
	sem := make(chan struct{}, maxQuoteConcurrency)
	var wg sync.WaitGroup

	for i := range positions {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			quoteCtx, cancel := context.WithTimeout(ctx, perQuoteTimeout)
			defer cancel()

			price, source, failure := resolvePositionPrice(quoteCtx, resolver, positions[i])
			results[i] = quoteResult{price: price, source: source, failure: failure}
		}()
	}
	wg.Wait()

	for i, p := range positions {
		res := results[i]
		cost := p.Quantity * p.AvgCost
		value := p.Quantity * res.price
		pnl := value - cost
		pnlPct := 0.0
		if cost != 0 {
			pnlPct = pnl / cost
		}

		snapshot.Positions[i] = PortfolioSnapshotPosition{
			ID:               p.ID,
			AccountID:        p.AccountID,
			Instrument:       p.Instrument,
			Quantity:         p.Quantity,
			AvgCost:          p.AvgCost,
			MarketPrice:      p.MarketPrice,
			OperationGuideID: p.OperationGuideID,
			CurrentPrice:     res.price,
			PriceSource:      res.source,
			Cost:             cost,
			MarketValue:      value,
			PnL:              pnl,
			PnLPct:           pnlPct,
			CreatedAt:        p.CreatedAt,
			UpdatedAt:        p.UpdatedAt,
		}
		snapshot.Summary.TotalCost += cost
		snapshot.Summary.TotalValue += value

		if res.source != portfolioPriceSourceRealtime {
			snapshot.HasStalePrices = true
		}
		if strings.TrimSpace(res.failure) != "" {
			snapshot.PriceFailures = append(snapshot.PriceFailures, PortfolioSnapshotPriceFailure{
				Symbol:    p.Instrument.Symbol,
				Name:      p.Instrument.Name,
				AssetType: p.Instrument.AssetType,
				Reason:    res.failure,
			})
		}
	}

	snapshot.Summary.TotalPnL = snapshot.Summary.TotalValue - snapshot.Summary.TotalCost
	if snapshot.Summary.TotalCost != 0 {
		snapshot.Summary.PnLPct = snapshot.Summary.TotalPnL / snapshot.Summary.TotalCost
	}

	return snapshot, nil
}

func resolvePositionPrice(ctx context.Context, resolver PortfolioPriceResolver, position investment.Position) (float64, string, string) {
	if resolver == nil {
		return fallbackPositionPrice(position, "price_resolver_not_initialized")
	}

	candidates := quoteCodeCandidates(position.Instrument)
	if len(candidates) == 0 {
		return fallbackPositionPrice(position, "missing_symbol")
	}

	assetType := strings.ToLower(strings.TrimSpace(position.Instrument.AssetType))
	failures := make([]string, 0, len(candidates))
	for _, code := range candidates {
		switch {
		case strings.Contains(assetType, "stock"), strings.Contains(assetType, "股票"):
			price, err := resolver.ResolveStockPrice(ctx, code)
			if err == nil && price > 0 {
				return price, portfolioPriceSourceRealtime, ""
			}
			failures = append(failures, fmt.Sprintf("%s=>%s", code, quoteErrorMessage("stock", err)))
		case strings.Contains(assetType, "fund"), strings.Contains(assetType, "基金"):
			price, err := resolver.ResolveFundPrice(ctx, code)
			if err == nil && price > 0 {
				return price, portfolioPriceSourceRealtime, ""
			}
			failures = append(failures, fmt.Sprintf("%s=>%s", code, quoteErrorMessage("fund", err)))
		default:
			stockPrice, stockErr := resolver.ResolveStockPrice(ctx, code)
			if stockErr == nil && stockPrice > 0 {
				return stockPrice, portfolioPriceSourceRealtime, ""
			}
			fundPrice, fundErr := resolver.ResolveFundPrice(ctx, code)
			if fundErr == nil && fundPrice > 0 {
				return fundPrice, portfolioPriceSourceRealtime, ""
			}
			failures = append(failures, fmt.Sprintf("%s=>%s", code, joinQuoteErrors(
				quoteErrorMessage("stock", stockErr),
				quoteErrorMessage("fund", fundErr),
			)))
		}
	}
	return fallbackPositionPrice(position, buildQuoteFailure(candidates, failures))
}

func fallbackPositionPrice(position investment.Position, failure string) (float64, string, string) {
	if position.MarketPrice != nil && *position.MarketPrice > 0 {
		return *position.MarketPrice, portfolioPriceSourceStored, strings.TrimSpace(failure)
	}
	return position.AvgCost, portfolioPriceSourceAvgCost, strings.TrimSpace(failure)
}

func quoteErrorMessage(prefix string, err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return ""
	}
	if prefix == "" {
		return msg
	}
	return prefix + "_quote_failed: " + msg
}

func joinQuoteErrors(messages ...string) string {
	out := make([]string, 0, len(messages))
	for _, msg := range messages {
		trimmed := strings.TrimSpace(msg)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, "; ")
}

func (r defaultPortfolioPriceResolver) ResolveStockPrice(ctx context.Context, symbol string) (float64, error) {
	result, err := r.marketData.Execute(ctx, map[string]interface{}{
		"action": "get_stock_intraday",
		"code":   strings.TrimSpace(symbol),
	})
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(result.Error) != "" {
		return 0, errors.New(result.Error)
	}
	return parseStockIntradayPrice(result.Output)
}

func (r defaultPortfolioPriceResolver) ResolveFundPrice(ctx context.Context, symbol string) (float64, error) {
	result, err := r.marketData.Execute(ctx, map[string]interface{}{
		"action": "get_fund_intraday",
		"code":   strings.TrimSpace(symbol),
	})
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(result.Error) != "" {
		return 0, errors.New(result.Error)
	}
	return parseFundIntradayPrice(result.Output)
}

func parseFundIntradayPrice(output interface{}) (float64, error) {
	switch points := output.(type) {
	case []IntradayPoint:
		return pickLastPositiveIntradayPrice(points)
	}

	raw, err := json.Marshal(output)
	if err != nil {
		return 0, err
	}
	var decoded []IntradayPoint
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return 0, err
	}
	return pickLastPositiveIntradayPrice(decoded)
}

func parseStockIntradayPrice(output interface{}) (float64, error) {
	switch points := output.(type) {
	case []IntradayPoint:
		return pickLastPositiveIntradayPrice(points)
	}

	raw, err := json.Marshal(output)
	if err != nil {
		return 0, err
	}
	var decoded []IntradayPoint
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return 0, err
	}
	return pickLastPositiveIntradayPrice(decoded)
}

func pickLastPositiveIntradayPrice(points []IntradayPoint) (float64, error) {
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Price > 0 {
			return points[i].Price, nil
		}
	}
	return 0, errors.New("empty_intraday_price")
}

func quoteCodeCandidates(instrument investment.Instrument) []string {
	candidates := make([]string, 0, 2)
	seen := map[string]struct{}{}
	add := func(code string) {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			return
		}
		key := strings.ToUpper(trimmed)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, trimmed)
	}

	add(instrument.Symbol)
	name := strings.TrimSpace(instrument.Name)
	if looksLikeQuoteCode(name) {
		add(name)
	}
	return candidates
}

func buildQuoteFailure(candidates []string, failures []string) string {
	list := strings.Join(candidates, ",")
	if len(failures) == 0 {
		return "quote_candidates=" + list + "; quote_failed"
	}
	return "quote_candidates=" + list + "; " + strings.Join(failures, " | ")
}

func normalizeInstrumentIdentity(symbol string, name string) (string, string) {
	normalizedSymbol := strings.TrimSpace(symbol)
	normalizedName := strings.TrimSpace(name)
	if !looksLikeQuoteCode(normalizedSymbol) && looksLikeQuoteCode(normalizedName) {
		return normalizedName, normalizedSymbol
	}
	return normalizedSymbol, normalizedName
}

func searchStockInstrumentByCode(ctx context.Context, provider instrumentSearchProvider, query string) *SearchItem {
	if provider == nil {
		return nil
	}
	symbol := normalizeSearchSymbol(query)
	if !looksLikeQuoteCode(symbol) {
		return nil
	}
	item, err := provider.SearchStockByCode(ctx, symbol)
	if err == nil && item != nil {
		normalized, ok := normalizeSearchItem(*item, "stock")
		if ok {
			return &normalized
		}
	}
	if looksLikeStrongStockCode(symbol) {
		return &SearchItem{
			Code: symbol,
			Name: symbol,
			Type: "stock",
		}
	}
	return nil
}

func buildSearchInstrumentRecords(stock *SearchItem, funds []SearchItem, limit int) []map[string]interface{} {
	if limit <= 0 {
		limit = 20
	}
	items := make([]SearchItem, 0, len(funds)+1)
	if stock != nil {
		items = append(items, *stock)
	}
	for _, fund := range funds {
		normalized, ok := normalizeSearchItem(fund, "fund")
		if !ok {
			continue
		}
		items = append(items, normalized)
	}

	records := make([]map[string]interface{}, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if len(records) >= limit {
			break
		}
		normalized, ok := normalizeSearchItem(item, "")
		if !ok {
			continue
		}
		key := strings.ToUpper(normalized.Type + ":" + normalized.Code)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		record := map[string]interface{}{
			"symbol":     normalized.Code,
			"name":       normalized.Name,
			"type":       normalized.Type,
			"asset_type": normalized.Type,
		}
		if normalized.Price > 0 {
			record["price"] = normalized.Price
		}
		records = append(records, record)
	}
	return records
}

func normalizeSearchItem(item SearchItem, defaultType string) (SearchItem, bool) {
	normalizedType := normalizeSearchAssetType(item.Type)
	if normalizedType == "unknown" {
		normalizedType = normalizeSearchAssetType(defaultType)
	}
	if normalizedType == "unknown" {
		return SearchItem{}, false
	}
	normalizedCode := normalizeSearchSymbol(item.Code)
	if normalizedCode == "" {
		return SearchItem{}, false
	}
	normalizedName := strings.TrimSpace(item.Name)
	if normalizedName == "" {
		normalizedName = normalizedCode
	}
	return SearchItem{
		Code:      normalizedCode,
		Name:      normalizedName,
		Type:      normalizedType,
		Price:     item.Price,
		Change:    item.Change,
		ChangePct: item.ChangePct,
	}, true
}

func normalizeSearchAssetType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "fund", "基金":
		return "fund"
	case "stock", "股票":
		return "stock"
	default:
		return "unknown"
	}
}

func normalizeSearchSymbol(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if len(normalized) == 8 {
		prefix := normalized[:2]
		if (prefix == "SH" || prefix == "SZ" || prefix == "BJ") && isAllDigits(normalized[2:]) {
			return normalized[2:]
		}
	}
	return normalized
}

func looksLikeStrongStockCode(value string) bool {
	normalized := normalizeSearchSymbol(value)
	if len(normalized) != 6 || !isAllDigits(normalized) {
		return false
	}
	switch {
	case strings.HasPrefix(normalized, "000"),
		strings.HasPrefix(normalized, "001"),
		strings.HasPrefix(normalized, "002"),
		strings.HasPrefix(normalized, "003"),
		strings.HasPrefix(normalized, "200"),
		strings.HasPrefix(normalized, "300"),
		strings.HasPrefix(normalized, "301"),
		strings.HasPrefix(normalized, "600"),
		strings.HasPrefix(normalized, "601"),
		strings.HasPrefix(normalized, "603"),
		strings.HasPrefix(normalized, "605"),
		strings.HasPrefix(normalized, "688"),
		strings.HasPrefix(normalized, "689"),
		strings.HasPrefix(normalized, "900"),
		strings.HasPrefix(normalized, "8"),
		strings.HasPrefix(normalized, "9"):
		return true
	default:
		return false
	}
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

func resolveUserID(ctx context.Context, svc *investment.Service, args map[string]interface{}) (int64, error) {
	userID, err := optionalInt64(args, "user_id")
	if err != nil {
		return 0, err
	}
	if userID != 0 {
		return userID, nil
	}
	if svc == nil {
		return 0, errors.New("service_not_initialized")
	}
	user, _, err := svc.EnsureDefaultAccount(ctx)
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}

func resolveAccountID(ctx context.Context, svc *investment.Service, args map[string]interface{}) (int64, error) {
	accountID, err := optionalInt64(args, "account_id")
	if err != nil {
		return 0, err
	}
	if accountID != 0 {
		return accountID, nil
	}
	if svc == nil {
		return 0, errors.New("service_not_initialized")
	}
	return svc.DefaultAccountID(ctx)
}

func requireString(args map[string]interface{}, key string) (string, error) {
	v := optionalString(args, key)
	if v == "" {
		return "", errors.New("missing_param_" + key)
	}
	return v, nil
}

func optionalString(args map[string]interface{}, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func requireInt64(args map[string]interface{}, key string) (int64, error) {
	v, err := optionalInt64(args, key)
	if err != nil {
		return 0, err
	}
	if v == 0 {
		return 0, errors.New("missing_param_" + key)
	}
	return v, nil
}

func optionalInt64(args map[string]interface{}, key string) (int64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, nil
	}
	switch t := v.(type) {
	case float64:
		return int64(t), nil
	case int:
		return int64(t), nil
	case int64:
		return t, nil
	case string:
		i, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, errors.New("invalid_param_" + key)
		}
		return i, nil
	default:
		return 0, errors.New("invalid_param_" + key)
	}
}

func optionalInt(args map[string]interface{}, key string) (int, error) {
	v, err := optionalInt64(args, key)
	return int(v), err
}

func requireFloat64(args map[string]interface{}, key string) (float64, error) {
	v, err := optionalFloat64Value(args, key)
	if err != nil {
		return 0, err
	}
	if v == 0 {
		return 0, errors.New("missing_param_" + key)
	}
	return v, nil
}

func optionalFloat64(args map[string]interface{}, key string) (*float64, error) {
	if _, ok := args[key]; !ok {
		return nil, nil
	}
	v, err := optionalFloat64Value(args, key)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func optionalFloat64Value(args map[string]interface{}, key string) (float64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, nil
	}
	switch t := v.(type) {
	case float64:
		return t, nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, errors.New("invalid_param_" + key)
		}
		return f, nil
	default:
		return 0, errors.New("invalid_param_" + key)
	}
}

func optionalTime(args map[string]interface{}, key string) (time.Time, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return time.Time{}, nil
	}
	switch t := v.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return time.Time{}, errors.New("invalid_param_" + key)
		}
		return parsed, nil
	default:
		return time.Time{}, errors.New("invalid_param_" + key)
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
