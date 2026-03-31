package dashboard

import (
	"context"
	"sort"
	"time"

	"genFu/internal/investment"
)

// DashboardData holds all data needed for rendering the dashboard.
type DashboardData struct {
	AccountID     int64            `json:"account_id"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Summary       SummaryKPI       `json:"summary"`
	Positions     []PositionDetail `json:"positions"`
	IndustryBreak []IndustryGroup  `json:"industry_breakdown"`
	AssetBreak    []AssetGroup     `json:"asset_breakdown"`
	Valuations    []ValuationPoint `json:"valuations"`
}

// SummaryKPI holds top-level KPI card values.
type SummaryKPI struct {
	TotalValue    float64 `json:"total_value"`
	TotalCost     float64 `json:"total_cost"`
	TotalPnL      float64 `json:"total_pnl"`
	TotalPnLPct   float64 `json:"total_pnl_pct"`
	PositionCount int     `json:"position_count"`
	CashBalance   float64 `json:"cash_balance"`
	DailyPnL      float64 `json:"daily_pnl"`
	DailyPnLPct   float64 `json:"daily_pnl_pct"`
}

// PositionDetail is a single position's full info for chart rendering.
type PositionDetail struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name"`
	AssetType   string  `json:"asset_type"`
	Industry    string  `json:"industry"`
	Quantity    float64 `json:"quantity"`
	AvgCost     float64 `json:"avg_cost"`
	MarketPrice float64 `json:"market_price"`
	Cost        float64 `json:"cost"`
	Value       float64 `json:"value"`
	PnL         float64 `json:"pnl"`
	PnLPct      float64 `json:"pnl_pct"`
	Weight      float64 `json:"weight"`
	DailyChange float64 `json:"daily_change"`
}

// IndustryGroup groups positions by industry.
type IndustryGroup struct {
	Industry   string           `json:"industry"`
	TotalValue float64          `json:"total_value"`
	TotalPnL   float64          `json:"total_pnl"`
	Weight     float64          `json:"weight"`
	Positions  []PositionDetail `json:"positions"`
}

// AssetGroup groups positions by asset type, with nested industry groups.
type AssetGroup struct {
	AssetType  string          `json:"asset_type"`
	TotalValue float64         `json:"total_value"`
	Weight     float64         `json:"weight"`
	Industries []IndustryGroup `json:"industries"`
}

// ValuationPoint is a single time-series data point.
type ValuationPoint struct {
	Date       string  `json:"date"`
	TotalValue float64 `json:"total_value"`
	TotalCost  float64 `json:"total_cost"`
	TotalPnL   float64 `json:"total_pnl"`
}

// DashboardOptions controls dashboard generation.
type DashboardOptions struct {
	ValuationDays int    `json:"valuation_days"`
	ColorScheme   string `json:"color_scheme"`
	IncludeCash   bool   `json:"include_cash"`
	GroupBy       string `json:"group_by"`
}

// DataService aggregates data from investment services for the dashboard.
type DataService struct {
	investSvc  *investment.Service
	investRepo *investment.Repository
}

// NewDataService creates a new DataService.
func NewDataService(svc *investment.Service, repo *investment.Repository) *DataService {
	return &DataService{investSvc: svc, investRepo: repo}
}

// BuildDashboardData constructs the complete DashboardData for one account.
func (ds *DataService) BuildDashboardData(ctx context.Context, accountID int64, opts DashboardOptions) (DashboardData, error) {
	if opts.ValuationDays <= 0 {
		opts.ValuationDays = 30
	}

	// 1. Analyze PnL (includes per-position cost/value/pnl)
	pnl, err := ds.investSvc.AnalyzePnL(ctx, accountID)
	if err != nil {
		return DashboardData{}, err
	}

	// 2. Get raw positions for Instrument metadata (industry, asset_type)
	rawPositions, err := ds.investSvc.ListPositions(ctx, accountID)
	if err != nil {
		return DashboardData{}, err
	}
	instrLookup := make(map[int64]investment.Instrument, len(rawPositions))
	for _, p := range rawPositions {
		instrLookup[p.Instrument.ID] = p.Instrument
	}

	// 3. Cash balance
	cashBalance, _ := ds.investRepo.EstimateAvailableCash(ctx, accountID)

	// 4. Valuation history
	valuations, _ := ds.investRepo.ListValuations(ctx, accountID, opts.ValuationDays)

	// Build PositionDetail list
	totalValue := pnl.TotalValue
	if totalValue == 0 {
		totalValue = 1 // guard against division by zero
	}

	posDetails := make([]PositionDetail, 0, len(pnl.Positions))
	for _, pp := range pnl.Positions {
		instr := instrLookup[pp.InstrumentID]
		industry := instr.Industry
		if industry == "" {
			industry = "未知行业"
		}
		assetType := instr.AssetType
		if assetType == "" {
			assetType = "stock"
		}

		posDetails = append(posDetails, PositionDetail{
			Symbol:      pp.Symbol,
			Name:        pp.Name,
			AssetType:   assetType,
			Industry:    industry,
			Quantity:    pp.Quantity,
			AvgCost:     pp.AvgCost,
			MarketPrice: pp.MarketPrice,
			Cost:        pp.Cost,
			Value:       pp.Value,
			PnL:         pp.PnL,
			PnLPct:      pp.PnLPct,
			Weight:      pp.Value / totalValue,
			DailyChange: 0, // not yet available
		})
	}

	// Sort by market value descending
	sort.Slice(posDetails, func(i, j int) bool {
		return posDetails[i].Value > posDetails[j].Value
	})

	// Build industry breakdown
	industryBreak := buildIndustryBreakdown(posDetails, totalValue)

	// Build asset-type breakdown with nested industries
	assetBreak := buildAssetBreakdown(posDetails, totalValue)

	// Build valuation points
	valPoints := make([]ValuationPoint, 0, len(valuations))
	for _, v := range valuations {
		valPoints = append(valPoints, ValuationPoint{
			Date:       v.ValuationAt.Format("2006-01-02"),
			TotalValue: v.TotalValue,
			TotalCost:  v.TotalCost,
			TotalPnL:   v.TotalPnL,
		})
	}

	// Summary KPI
	pnlPct := 0.0
	if pnl.TotalCost != 0 {
		pnlPct = pnl.TotalPnL / pnl.TotalCost
	}

	return DashboardData{
		AccountID:   accountID,
		GeneratedAt: time.Now(),
		Summary: SummaryKPI{
			TotalValue:    pnl.TotalValue,
			TotalCost:     pnl.TotalCost,
			TotalPnL:      pnl.TotalPnL,
			TotalPnLPct:   pnlPct,
			PositionCount: len(pnl.Positions),
			CashBalance:   cashBalance,
		},
		Positions:     posDetails,
		IndustryBreak: industryBreak,
		AssetBreak:    assetBreak,
		Valuations:    valPoints,
	}, nil
}

func buildIndustryBreakdown(positions []PositionDetail, totalValue float64) []IndustryGroup {
	m := make(map[string]*IndustryGroup)
	for _, pd := range positions {
		ig, ok := m[pd.Industry]
		if !ok {
			ig = &IndustryGroup{Industry: pd.Industry}
			m[pd.Industry] = ig
		}
		ig.Positions = append(ig.Positions, pd)
		ig.TotalValue += pd.Value
		ig.TotalPnL += pd.PnL
	}

	result := make([]IndustryGroup, 0, len(m))
	for _, ig := range m {
		ig.Weight = ig.TotalValue / totalValue
		result = append(result, *ig)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalValue > result[j].TotalValue
	})
	return result
}

func buildAssetBreakdown(positions []PositionDetail, totalValue float64) []AssetGroup {
	// First group by asset type
	assetMap := make(map[string][]PositionDetail)
	assetValue := make(map[string]float64)
	for _, pd := range positions {
		assetMap[pd.AssetType] = append(assetMap[pd.AssetType], pd)
		assetValue[pd.AssetType] += pd.Value
	}

	result := make([]AssetGroup, 0, len(assetMap))
	for at, pds := range assetMap {
		ag := AssetGroup{
			AssetType:  at,
			TotalValue: assetValue[at],
			Weight:     assetValue[at] / totalValue,
		}

		// Nest industries inside each asset group
		indMap := make(map[string]*IndustryGroup)
		for _, pd := range pds {
			ig, ok := indMap[pd.Industry]
			if !ok {
				ig = &IndustryGroup{Industry: pd.Industry}
				indMap[pd.Industry] = ig
			}
			ig.Positions = append(ig.Positions, pd)
			ig.TotalValue += pd.Value
			ig.TotalPnL += pd.PnL
		}
		for _, ig := range indMap {
			ig.Weight = ig.TotalValue / totalValue
			ag.Industries = append(ag.Industries, *ig)
		}
		sort.Slice(ag.Industries, func(i, j int) bool {
			return ag.Industries[i].TotalValue > ag.Industries[j].TotalValue
		})

		result = append(result, ag)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalValue > result[j].TotalValue
	})
	return result
}
