package tool

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"genFu/internal/investment"
)

type InvestmentTool struct {
	svc        *investment.Service
	eastMoney  EastMoneyTool
	marketData MarketDataTool
}

func NewInvestmentTool(svc *investment.Service) InvestmentTool {
	return NewInvestmentToolWithEastMoney(svc, NewEastMoneyTool())
}

func NewInvestmentToolWithEastMoney(svc *investment.Service, eastMoney EastMoneyTool) InvestmentTool {
	return InvestmentTool{
		svc:        svc,
		eastMoney:  eastMoney,
		marketData: NewMarketDataTool(svc),
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
		// 优先使用外部API搜索
		externalResults, err := t.eastMoney.SearchInstruments(ctx, query, limit)
		if err == nil && len(externalResults) > 0 {
			// 转换为统一格式
			instruments := make([]map[string]interface{}, 0, len(externalResults))
			for _, item := range externalResults {
				instruments = append(instruments, map[string]interface{}{
					"symbol":     item.Code,
					"name":       item.Name,
					"asset_type": item.Type,
					"price":      item.Price,
				})
			}
			return ToolResult{Name: "investment", Output: instruments, Error: ""}, nil
		}
		// 如果外部API失败，降级到本地数据库搜索
		instruments, err := t.svc.SearchInstruments(ctx, query, limit)
		return ToolResult{Name: "investment", Output: instruments, Error: errorString(err)}, err
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
	default:
		return ToolResult{Name: "investment", Error: "unsupported_action"}, errors.New("unsupported_action")
	}
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
