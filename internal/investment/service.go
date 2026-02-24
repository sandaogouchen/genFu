package investment

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) EnsureDefaultAccount(ctx context.Context) (User, Account, error) {
	if s == nil || s.repo == nil {
		return User{}, Account{}, errors.New("service_not_initialized")
	}
	return s.repo.EnsureDefaultAccount(ctx)
}

func (s *Service) DefaultAccountID(ctx context.Context) (int64, error) {
	_, account, err := s.EnsureDefaultAccount(ctx)
	if err != nil {
		return 0, err
	}
	return account.ID, nil
}

func (s *Service) CreateUser(ctx context.Context, name string) (User, error) {
	return s.repo.CreateUser(ctx, name)
}

func (s *Service) CreateAccount(ctx context.Context, userID int64, name string, baseCurrency string) (Account, error) {
	return s.repo.CreateAccount(ctx, userID, name, baseCurrency)
}

func (s *Service) UpsertInstrument(ctx context.Context, symbol string, name string, assetType string) (Instrument, error) {
	return s.repo.UpsertInstrument(ctx, symbol, name, assetType)
}

func (s *Service) SetPosition(ctx context.Context, accountID int64, instrumentID int64, quantity float64, avgCost float64, marketPrice *float64) (Position, error) {
	return s.repo.SetPosition(ctx, accountID, instrumentID, quantity, avgCost, marketPrice)
}

func (s *Service) RecordTrade(ctx context.Context, accountID int64, instrumentID int64, side string, quantity float64, price float64, fee float64, tradeAt time.Time, note string) (Trade, Position, error) {
	normalized := strings.ToLower(strings.TrimSpace(side))
	switch normalized {
	case "buy", "sell":
	default:
		return Trade{}, Position{}, errors.New("invalid_side")
	}
	return s.repo.RecordTradeAndUpdatePosition(ctx, accountID, instrumentID, normalized, quantity, price, fee, tradeAt, note)
}

func (s *Service) RecordCashFlow(ctx context.Context, accountID int64, amount float64, currency string, flowType string, flowAt time.Time, note string) (CashFlow, error) {
	return s.repo.RecordCashFlow(ctx, accountID, amount, currency, flowType, flowAt, note)
}

func (s *Service) RecordValuation(ctx context.Context, accountID int64, totalValue float64, totalCost float64, valuationAt time.Time) (Valuation, error) {
	return s.repo.RecordValuation(ctx, accountID, totalValue, totalCost, valuationAt)
}

func (s *Service) ListPositions(ctx context.Context, accountID int64) ([]Position, error) {
	return s.repo.ListPositions(ctx, accountID)
}

func (s *Service) GetPosition(ctx context.Context, accountID int64, instrumentID int64) (Position, error) {
	return s.repo.GetPosition(ctx, accountID, instrumentID)
}

func (s *Service) DeletePosition(ctx context.Context, accountID int64, instrumentID int64) error {
	return s.repo.DeletePosition(ctx, accountID, instrumentID)
}

func (s *Service) ListTrades(ctx context.Context, accountID int64, limit int, offset int) ([]Trade, error) {
	return s.repo.ListTrades(ctx, accountID, limit, offset)
}

func (s *Service) GetPortfolioSummary(ctx context.Context, accountID int64) (PortfolioSummary, error) {
	return s.repo.GetPortfolioSummary(ctx, accountID)
}

func (s *Service) AnalyzePnL(ctx context.Context, accountID int64) (AccountPnL, error) {
	positions, err := s.repo.ListPositions(ctx, accountID)
	if err != nil {
		return AccountPnL{}, err
	}
	result := AccountPnL{
		AccountID: accountID,
		Positions: make([]PositionPnL, 0, len(positions)),
	}
	for _, p := range positions {
		market := p.AvgCost
		if p.MarketPrice != nil {
			market = *p.MarketPrice
		}
		cost := p.Quantity * p.AvgCost
		value := p.Quantity * market
		pnl := value - cost
		pct := 0.0
		if cost != 0 {
			pct = pnl / cost
		}
		result.Positions = append(result.Positions, PositionPnL{
			AccountID:    p.AccountID,
			InstrumentID: p.Instrument.ID,
			Symbol:       p.Instrument.Symbol,
			Name:         p.Instrument.Name,
			Quantity:     p.Quantity,
			AvgCost:      p.AvgCost,
			MarketPrice:  market,
			Cost:         cost,
			Value:        value,
			PnL:          pnl,
			PnLPct:       pct,
		})
		result.TotalCost += cost
		result.TotalValue += value
	}
	result.TotalPnL = result.TotalValue - result.TotalCost
	return result, nil
}

func (s *Service) SearchInstruments(ctx context.Context, query string, limit int) ([]Instrument, error) {
	return s.repo.SearchInstruments(ctx, query, limit)
}

func (s *Service) GetOrCreateInstrumentBySymbol(ctx context.Context, symbol string, name string, assetType string) (Instrument, error) {
	if instrument, err := s.repo.GetInstrumentBySymbol(ctx, symbol); err == nil {
		return instrument, nil
	}
	return s.repo.UpsertInstrument(ctx, symbol, name, assetType)
}

// AddPositionByValue 按持有金额添加持仓
// value: 持有金额
// avgCost: 买入成本价（可选，如果不提供则用当前市价）
// marketPrice: 当前市价
func (s *Service) AddPositionByValue(ctx context.Context, accountID int64, symbol string, name string, assetType string, value float64, avgCost *float64, marketPrice float64) (Position, error) {
	instrument, err := s.GetOrCreateInstrumentBySymbol(ctx, symbol, name, assetType)
	if err != nil {
		return Position{}, err
	}
	cost := marketPrice
	if avgCost != nil {
		cost = *avgCost
	}
	quantity := value / marketPrice
	return s.repo.SetPosition(ctx, accountID, instrument.ID, quantity, cost, &marketPrice)
}

// AddPositionByCost 按买入成本添加持仓
// cost: 买入总成本
// avgCost: 买入成本价
// marketPrice: 当前市价（可选）
func (s *Service) AddPositionByCost(ctx context.Context, accountID int64, symbol string, name string, assetType string, cost float64, avgCost float64, marketPrice *float64) (Position, error) {
	instrument, err := s.GetOrCreateInstrumentBySymbol(ctx, symbol, name, assetType)
	if err != nil {
		return Position{}, err
	}
	quantity := cost / avgCost
	return s.repo.SetPosition(ctx, accountID, instrument.ID, quantity, avgCost, marketPrice)
}

// AddPositionByPnL 按当前盈利添加持仓
// pnl: 当前盈利金额
// avgCost: 买入成本价
// marketPrice: 当前市价
func (s *Service) AddPositionByPnL(ctx context.Context, accountID int64, symbol string, name string, assetType string, pnl float64, avgCost float64, marketPrice float64) (Position, error) {
	instrument, err := s.GetOrCreateInstrumentBySymbol(ctx, symbol, name, assetType)
	if err != nil {
		return Position{}, err
	}
	// 盈利 = 数量 * (市价 - 成本价)
	// 数量 = 盈利 / (市价 - 成本价)
	priceDiff := marketPrice - avgCost
	if priceDiff == 0 {
		return Position{}, errors.New("market_price_equal_to_avg_cost")
	}
	quantity := pnl / priceDiff
	return s.repo.SetPosition(ctx, accountID, instrument.ID, quantity, avgCost, &marketPrice)
}

// AddPositionSimple 简化添加持仓
// cost: 购入总成本
// currentValue: 当前总金额/市值
// marketPrice: 当前单价（市价），如果为0则自动获取
// 计算逻辑：
// - 数量 = 当前金额 / 当前单价
// - 成本价 = 购入总成本 / 数量
func (s *Service) AddPositionSimple(ctx context.Context, accountID int64, symbol string, name string, assetType string, cost float64, currentValue float64, marketPrice float64) (Position, error) {
	instrument, err := s.GetOrCreateInstrumentBySymbol(ctx, symbol, name, assetType)
	if err != nil {
		return Position{}, err
	}

	// 如果没有提供市价，使用默认值
	if marketPrice <= 0 {
		marketPrice = 1.0
	}

	// 计算数量：���前金额 / 当前单价
	quantity := currentValue / marketPrice

	// 计算成本价：总成本 / 数量
	avgCost := cost
	if quantity > 0 {
		avgCost = cost / quantity
	}

	return s.repo.SetPosition(ctx, accountID, instrument.ID, quantity, avgCost, &marketPrice)
}
